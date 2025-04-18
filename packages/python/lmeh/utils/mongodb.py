import json
import logging
from copy import deepcopy
from dataclasses import asdict
from datetime import datetime
from typing import List, Tuple

from app.app import get_app_logger
from bson.objectid import ObjectId
from lm_eval.api.instance import Instance

from packages.python.common.mongodb import MongoClient
from packages.python.lmeh.utils.mongo_aggrs import (
    aggregate_doc_ids,
    aggregate_response_tree,
    aggregate_old_tasks,
    aggregate_supplier_task_results,
)
from packages.python.protocol.protocol import (
    CompletionRequest,
    CompletionResponse,
    PocketNetworkMongoDBPrompt,
    PocketNetworkMongoDBResultBase,
    PocketNetworkMongoDBResultNumerical,
    PocketNetworkMongoDBTask,
)

eval_logger = get_app_logger("sample")
evaluation_logger = get_app_logger("evaluation")
summarization_logger = get_app_logger("summarize_taxonomy")


class MongoOperator:
    def __init__(self, client: MongoClient, collections_map=None):
        if collections_map is None:
            collections_map = {}

        self.client = client
        # try to read the rewrite collection name or use the default one
        # avoiding pass it on every call if not need
        self.tokenizers_collection = (
            collections_map["tokenizers"]
            if "tokenizers" in collections_map
            else "tokenizers"
        )
        self.configs_collection = (
            collections_map["configs"] if "configs" in collections_map else "configs"
        )
        self.suppliers_collection = (
            collections_map["suppliers"] if "suppliers" in collections_map else "suppliers"
        )
        self.tasks_collection = (
            collections_map["tasks"] if "tasks" in collections_map else "tasks"
        )
        self.instances_collection = (
            collections_map["instances"]
            if "instances" in collections_map
            else "instances"
        )
        self.prompts_collection = (
            collections_map["prompts"] if "prompts" in collections_map else "prompts"
        )
        self.responses_collection = (
            collections_map["responses"]
            if "responses" in collections_map
            else "responses"
        )
        self.results_collection = (
            collections_map["results"] if "results" in collections_map else "results"
        )
        self.buffers_numerical_collection = (
            collections_map["buffers_numerical"]
            if "buffers_numerical" in collections_map
            else "buffers_numerical"
        )
        self.buffers_signatures_collection = (
            collections_map["buffers_signatures"]
            if "buffers_signatures" in collections_map
            else "buffers_signatures"
        )
        self.taxonomy_summaries = (
            collections_map["taxonomy_summaries"]
            if "taxonomy_summaries" in collections_map
            else "taxonomy_summaries"
        )

    # TODO : This should reffer to PocketNetworkMongoDBInstance and not depend on LMEH blindly
    @staticmethod
    def instance_to_dict(instance: Instance, task_id: ObjectId) -> dict:
        instance_mongo = asdict(instance)
        instance_mongo.pop("resps", None)
        instance_mongo.pop("filtered_resps", None)
        instance_mongo["task_id"] = task_id
        instance_mongo["_id"] = ObjectId()
        instance_mongo["done"] = False
        return instance_mongo

    async def getnsupplier_id(self, address: str, service: str) -> str:
        supplier = await self.client.db[self.suppliers_collection].find_one(
            {"address": address, "service": service}
        )

        if supplier is None:
            eval_logger.error("Supplier address not found.", adress=address)
            raise RuntimeError(
                f"Supplier address {address} does not exist in the database."
            )

        eval_logger.debug("Supplier found.", supplier=supplier)

        # Get the supplier ID
        if supplier.get("_id", None) is None:
            eval_logger.error(
                "Supplier address has no _id, cannot load tokenizer hash.", adress=address
            )
            raise RuntimeError(
                f"Supplier address {address}, has no _id, cannot load tokenizer hash."
            )

        return supplier["_id"]

    async def get_signature_hash(
        self, address: str, supplier_id: str, signature_name: str
    ) -> str:
        # Get the corresponding signature buffer
        buffer = await self.client.db[self.buffers_signatures_collection].find_one(
            {
                "task_data.supplier_id": supplier_id,
                "task_data.framework": "signatures",
                "task_data.task": signature_name,
            }
        )

        if buffer is None:
            eval_logger.error(
                f"Buffer for {signature_name} signature not found.", adress=address
            )
            raise RuntimeError(
                f"Supplier address {address} does not have a {signature_name} signature buffer associated."
            )

        eval_logger.debug(f"{signature_name} signature buffer found.", buffer=buffer)

        this_hash = buffer.get("last_signature", None)
        if this_hash is None:
            eval_logger.error(
                "Buffer has no last signature field, entry is malformed cannot proceed.",
                adress=address,
            )
            raise RuntimeError(
                f"Supplier address {address} buffer has no last signature field, entry is malformed cannot proceed."
            )

        return this_hash

    async def get_tokenizer_hash(self, address: str, service: str) -> str:
        # Get supplier ID
        supplier_id = await self.get_supplier_id(address, service)
        # Get tokenizer signature hash
        tokenizer_hash = await self.get_signature_hash(address, supplier_id, "tokenizer")

        return tokenizer_hash

    async def get_config_hash(self, address: str, service: str) -> str:
        # Get supplier ID
        supplier_id = await self.get_supplier_id(address, service)
        # Get config signature hash
        config_hash = await self.get_signature_hash(address, supplier_id, "config")

        return config_hash

    async def get_tokenizer_entry(self, tokenizer_hash: str):
        return await self.client.db[self.tokenizers_collection].find_one(
            {"hash": tokenizer_hash}
        )

    async def get_config_entry(self, config_hash: str):
        return await self.client.db[self.configs_collection].find_one(
            {"hash": config_hash}
        )

    async def get_tokenizer_objects(
        self, address: str, service: str
    ) -> Tuple[bool, dict]:
        tokenizer_hash = await self.get_tokenizer_hash(address, service)

        if tokenizer_hash == "":
            eval_logger.warn(
                "Supplier address does not have a valid tokenizer_hash.", adress=address
            )
            return False, {}

        tokenizer_object = await self.get_tokenizer_entry(tokenizer_hash)

        # Validate that the tokenizer is not empty
        if tokenizer_object is None:
            eval_logger.error(
                "Tokenizer hash not found.", address=address, hash=tokenizer_hash
            )
            raise RuntimeError(
                f"Tokenizer with hash {tokenizer_hash} does not exist in the database."
            )

        tokenizer = tokenizer_object["tokenizer"]
        eval_logger.debug("Tokenizer found.", tokenizer_keys=list(tokenizer.keys()))

        if "model_max_length" in tokenizer["tokenizer_config"]:
            tokenizer["tokenizer_config"]["model_max_length"] = int(
                tokenizer["tokenizer_config"]["model_max_length"]
            )

        return True, tokenizer

    async def get_config_objects(self, address: str, service: str) -> Tuple[bool, dict]:
        # TODO
        # add get_config_hash method to
        config_hash = await self.get_config_hash(address, service)

        if config_hash == "":
            eval_logger.warn(
                "Supplier address does not have a valid config_hash.", adress=address
            )
            return False, {}

        config_object = await self.get_config_entry(config_hash)

        # Validate that the Config is not empty
        if config_object is None:
            eval_logger.error(
                "Config hash not found in MongoDB.", address=address, hash=config_hash
            )
            raise RuntimeError(
                f"Config with hash {config_hash} does not exist in the database."
            )
        eval_logger.debug("Config found.", config_keys=list(config_object.keys()))
        _config = config_object["config"]
        eval_logger.debug("Config found.", _config=list(_config.keys()))
        return True, _config

    async def get_prompt_request(self, request_id: ObjectId) -> CompletionRequest:
        prompt_doc = await self.client.db[self.prompts_collection].find_one(
            {"_id": request_id}
        )

        if prompt_doc is None:
            eval_logger.error("Prompt request not found.", request_id=request_id)
            raise RuntimeError(
                f"Prompt request with ID {request_id} does not exist in the database."
            )

        data = prompt_doc["data"]
        try:
            # handle the exception to bring a light on production debugging if needed.
            data = json.loads(data)
        except Exception as e:
            eval_logger.error("Bad JSON data format", data=data, error=str(e))
            raise RuntimeError("Bad JSON data format")

        request = CompletionRequest(**data)
        eval_logger.debug("Prompt request found.", request_id=request_id)

        return request

    ###############################################
    # Evaluator
    ################################################
    async def get_doc_ids_by_task(self, task_id: ObjectId) -> List[int]:
        # Create the aggregation pipeline with the given task_id
        aggr = aggregate_doc_ids(task_id)
        # Execute the aggregation
        cursor = self.client.db[self.instances_collection].aggregate(aggr)
        # get all of them
        result = await cursor.to_list(length=None)

        if len(result) == 0:
            evaluation_logger.error("Task ID not found.", task_id=task_id)
            raise RuntimeError(f"Task ID {task_id} does not exist in the database.")

        # Convert the result to a list and return it
        doc_ids = result[0]["doc_ids"]
        return doc_ids

    async def get_task(self, task_id: ObjectId):
        task = await self.client.db[self.tasks_collection].find_one({"_id": task_id})

        if task is None:
            evaluation_logger.error("Task ID not found.", task_id=task_id)
            raise RuntimeError(f"Task ID {task_id} does not exist in the database.")

        task.pop("_id", None)
        evaluation_logger.debug("Task:", task=task)
        task = PocketNetworkMongoDBTask(**task)
        task.id = task_id

        return task

    async def get_tasks(self):
        cursor = self.client.db[self.tasks_collection].find(
            {"done": True, "evaluated": False}
        )
        tasks = await cursor.to_list(length=None)
        return tasks

    async def get_old_tasks(self, blocks_ago=40):
        # Get latest response height
        # TODO : Change this with a parameter, that must come from the activity making query to the network
        cursor = self.client.db[self.responses_collection].aggregate(
            [{"$group": {"_id": None, "latest_height": {"$max": "$height"}}}]
        )
        latest_height = await cursor.to_list(length=None)
        if len(latest_height) == 0:
            return []  # Nothing found
        # Now get all tasks that have all prompts resolved since a while but
        # somehow the evaluation was not correctly triggered
        # (this can happen due to sessions changing and tasks being terminated)
        cursor = self.client.db[self.tasks_collection].aggregate(
            aggregate_old_tasks(latest_height[0]["latest_height"], blocks_ago)
        )
        tasks = await cursor.to_list(length=None)
        return tasks

    async def set_task_as_done(self, task_id):
        async with self.client.start_transaction() as session:
            try:
                await self.client.db[self.tasks_collection].find_one_and_update(
                    {"_id": task_id},
                    {"$set": {"done": True}},
                    session=session,
                )
            except Exception as e:
                raise RuntimeError(f"Error marking task as done: {str(e)}")

    async def retrieve_responses(
        self,
        task_id: ObjectId,
    ) -> List[str]:
        cursor = self.client.db[self.tasks_collection].aggregate(
            aggregate_response_tree(task_id)
        )
        result = await cursor.to_list(length=None)

        if len(result) == 0:
            evaluation_logger.error("Task ID not found.", task_id=task_id)
            raise RuntimeError(f"Task ID {task_id} does not exist in the database.")

        return result

    async def reconstruct_instances(
        self, task_id: ObjectId, eval_logger: logging.Logger
    ) -> List[Instance]:
        result = await self.retrieve_responses(task_id)

        valid_fields = {field.name for field in Instance.__dataclass_fields__.values()}
        instances = []
        failed_instances = []
        remove_doc_ids = set()
        kept_doc_ids = set()
        list_result_height = []
        for doc in result:
            i, p = doc["instance"], doc["prompt"]
            list_result_height.append(doc["response"]["session_height"])
            if not doc["response"]["ok"]:
                remove_doc_ids.add(i["doc_id"])
                failed_instances.append(
                    {
                        "id": i["doc_id"],
                        "code": doc["response"]["error_code"],
                        "error": doc["response"]["error"],
                    }
                )
                continue
            else:
                try:
                    # handle the exception to bring a light on production debugging if needed.
                    r = json.loads(doc["response"]["response"])
                    ms = int(doc["response"]["ms"])
                except Exception as e:
                    remove_doc_ids.add(i["doc_id"])
                    error_str = "Bad JSON data format (response)"
                    eval_logger.error(
                        error_str,
                        # response=doc["response"]["response"], # Spams log
                        errpr=str(e),
                    )
                    failed_instances.append(
                        {"id": i["doc_id"], "code": 11, "error": error_str}
                    )
                    continue
            instance_dict = {
                key: value for key, value in i.items() if key in valid_fields
            }
            instance = Instance(**instance_dict)
            instance.repeats = 1  # to avoid double evaluation for each instance
            p["id"] = deepcopy(p["_id"])
            p.pop("_id")
            instance.prompt = PocketNetworkMongoDBPrompt(**p)
            try:
                # handle the exception to bring a light on production debugging if needed.
                request_data = json.loads(instance.prompt.data)
            except Exception as e:
                remove_doc_ids.add(i["doc_id"])
                error_str = "Bad JSON data format (prompt)"
                eval_logger.error(
                    error_str,
                    prompt_data=instance.prompt.data,
                    error=str(e),
                )
                failed_instances.append(
                    {"id": i["doc_id"], "code": 11, "error": error_str}
                )
                continue
            instance.prompt.data = CompletionRequest(**request_data)

            try:
                r["response_time"] = ms
                instance.resp = CompletionResponse(**r)
            except Exception as e:
                remove_doc_ids.add(i["doc_id"])
                error_str = "Bad JSON CompletionResponse format"
                eval_logger.debug(  # This is rather common if we cannot control the supply.
                    error_str,
                    response=r,
                    error=str(e),
                )
                failed_instances.append(
                    {"id": i["doc_id"], "code": 11, "error": error_str}
                )
                continue

            instances.append(instance)

        result_height = max(list_result_height)

        if len(instances) == 0 and len(remove_doc_ids) > 0:
            return [], [], result_height, failed_instances

        # Remove uncompleted docs_ids
        if len(remove_doc_ids) > 0:
            instances = [i for i in instances if i.doc_id not in remove_doc_ids]
            for i in instances:
                kept_doc_ids.add(i.doc_id)

        instances = sorted(instances, key=lambda x: (x.doc_id, x.idx))

        return instances, sorted(list(kept_doc_ids)), result_height, failed_instances

    async def mark_task_to_drop(self, task_id: ObjectId):
        empty_result = PocketNetworkMongoDBResultNumerical(
            result_data=PocketNetworkMongoDBResultBase(
                task_id=task_id,
                status=11,
                num_samples=0,
                result_height=-1,
                result_time=datetime.today().isoformat(),
            ),
            scores=[],
        ).model_dump(by_alias=True)

        async with self.client.start_transaction() as session:
            try:
                await self.client.db[self.tasks_collection].find_one_and_update(
                    {"_id": task_id},
                    {"$set": {"drop": True}},
                    session=session,
                )
            except Exception as e:
                raise RuntimeError(f"Error marking task to drop: {str(e)}")

            try:
                await self.client.db[self.results_collection].insert_one(
                    empty_result,
                    session=session,
                )
            except Exception as e:
                raise RuntimeError(
                    f"Error setting the result in drop procedure: {str(e)}"
                )

    ###############################################
    # Summarizer
    ################################################

    async def get_suppliers(self):
        cursor = self.client.db[
            self.suppliers_collection
        ].find(
            {}  # TODO : Add filter for only "last_see_height" > 0 when that property is correctly tracked
        )
        suppliers = await cursor.to_list(length=None)
        return suppliers

    async def get_supplier_results_for_task(
        self, supplier_id: ObjectId, framework: str, task: str
    ) -> List[dict]:
        # Create the aggregation pipeline with the given task_id
        aggr = aggregate_supplier_task_results(supplier_id, framework, task)
        # Execute the aggregation
        cursor = self.client.db[self.buffers_numerical_collection].aggregate(aggr)
        # get all of them
        result = await cursor.to_list(length=None)

        return result
