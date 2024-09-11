import json
import logging
from copy import deepcopy
from dataclasses import asdict
from datetime import datetime
from typing import List

from app.app import get_app_logger
from bson.objectid import ObjectId
from lm_eval.api.instance import Instance
from temporalio.exceptions import ApplicationError

from packages.python.common.mongodb import MongoClient
from packages.python.lmeh.utils.mongo_aggrs import (
    aggregate_doc_ids,
    aggregate_response_tree,
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
        self.nodes_collection = (
            collections_map["nodes"] if "nodes" in collections_map else "nodes"
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

    async def get_tokenizer_hash(self, address: str, service: str) -> str:
        node = await self.client.db[self.nodes_collection].find_one(
            {"address": address, "service": service}
        )

        if node is None:
            eval_logger.error("Node address not found.", adress=address)
            raise ApplicationError(
                f"Node address {address} does not exist in the database."
            )

        eval_logger.debug("Node found.", node=node)

        # Get the node ID
        if node.get("_id", None) is None:
            eval_logger.error(
                "Node address has no _id, cannot load tokenizer hash.", adress=address
            )
            raise ApplicationError(
                f"Node address {address}, has no _id, cannot load tokenizer hash."
            )

        # Get the corresponding signature buffer
        buffer = await self.client.db[self.buffers_signatures_collection].find_one(
            {
                "task_data.node_id": node["_id"],
                "task_data.framework": "signatures",
                "task_data.task": "tokenizer",
            }
        )

        if buffer is None:
            eval_logger.error(
                "Buffer for tokenizer signature not found.", adress=address
            )
            raise ApplicationError(
                f"Node address {address} does not have a tokenizer signature buffer associated."
            )

        eval_logger.debug("Tokennizer signature buffer found.", buffer=buffer)

        tokenizer_hash = buffer.get("last_signature", None)
        if tokenizer_hash is None:
            eval_logger.error(
                "Buffer has no last signature field, entry is malformed cannot procede.",
                adress=address,
            )
            raise ApplicationError(
                f"Node address {address} buffer has no last signature field, entry is malformed cannot procede."
            )

        return tokenizer_hash

    async def get_tokenizer_entry(self, tokenizer_hash: str):
        return await self.client.db[self.tokenizers_collection].find_one(
            {"hash": tokenizer_hash}
        )

    async def get_config_entry(self, config_hash: str):
        return await self.client.db[self.configs_collection].find_one(
            {"hash": config_hash}
        )

    async def get_tokenizer_objects(self, address: str, service: str) -> dict:
        tokenizer_hash = await self.get_tokenizer_hash(address, service)

        if tokenizer_hash == "":
            eval_logger.error(
                "Node address does not have a valid tokenizer_hash.", adress=address
            )
            raise ApplicationError(
                f"Node address {address} does not have a valid tokenizer_hash."
            )

        tokenizer_object = await self.get_tokenizer_entry(tokenizer_hash)

        # Validate that the tokenizer is not empty
        if tokenizer_object is None:
            eval_logger.error(
                "Tokenizer hash not found.", address=address, hash=tokenizer_hash
            )
            raise ApplicationError(
                f"Tokenizer with hash {tokenizer_hash} does not exist in the database."
            )

        tokenizer = tokenizer_object["tokenizer"]
        eval_logger.debug("Tokenizer found.", tokenizer_keys=list(tokenizer.keys()))

        if "model_max_length" in tokenizer["tokenizer_config"]:
            tokenizer["tokenizer_config"]["model_max_length"] = int(
                tokenizer["tokenizer_config"]["model_max_length"]
            )

        return tokenizer

    async def get_config_objects(self, address: str, service: str) -> dict:
        # TODO
        # add get_config_hash method to
        config_hash = await self.get_tokenizer_hash(address, service)

        if config_hash == "":
            eval_logger.error(
                "Node address does not have a valid config_hash.", adress=address
            )
            raise ApplicationError(
                f"Node address {address} does not have a valid config_hash."
            )

        config_object = await self.get_config_entry(config_hash)

        # Validate that the Config is not empty
        if config_object is None:
            eval_logger.error(
                "Config hash not found.", address=address, hash=config_hash
            )
            raise ApplicationError(
                f"Config with hash {config_hash} does not exist in the database."
            )
        eval_logger.debug("Config found.", config_keys=list(config_object.keys()))
        _config = config_object["config"]
        eval_logger.debug("Config found.", _config=list(_config.keys()))
        return _config

    async def get_prompt_request(self, request_id: ObjectId) -> CompletionRequest:
        prompt_doc = await self.client.db[self.prompts_collection].find_one(
            {"_id": request_id}
        )

        if prompt_doc is None:
            eval_logger.error("Prompt request not found.", request_id=request_id)
            raise ApplicationError(
                f"Prompt request with ID {request_id} does not exist in the database."
            )

        data = prompt_doc["data"]
        try:
            # handle the exception to bring a light on production debugging if needed.
            data = json.loads(data)
        except Exception as e:
            raise ApplicationError(
                "Bad JSON data format",
                data,
                str(e),
                type="BadJSONFormat",
                non_retryable=True,
            )

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
            raise ApplicationError(
                f"Task ID {task_id} does not exist in the database.",
                str(task_id),
                type="TaskNotFound",
                non_retryable=False,
            )

        # Convert the result to a list and return it
        doc_ids = result[0]["doc_ids"]
        return doc_ids

    async def get_task(self, task_id: ObjectId):
        task = await self.client.db[self.tasks_collection].find_one({"_id": task_id})

        if task is None:
            evaluation_logger.error("Task ID not found.", task_id=task_id)
            raise ApplicationError(
                f"Task ID {task_id} does not exist in the database.",
                str(task_id),
                type="TaskNotFound",
                non_retryable=False,
            )

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
            raise ApplicationError(
                f"Task ID {task_id} does not exist in the database.",
                str(task_id),
                type="TaskNotFound",
                non_retryable=False,
            )

        return result

    async def reconstruct_instances(
        self, task_id: ObjectId, eval_logger: logging.Logger
    ) -> List[Instance]:
        result = await self.retrieve_responses(task_id)

        valid_fields = {field.name for field in Instance.__dataclass_fields__.values()}
        instances = []
        remove_doc_ids = set()
        kept_doc_ids = set()
        list_result_height = []
        for doc in result:
            i, p = doc["instance"], doc["prompt"]
            list_result_height.append(doc["response"]["session_height"])
            if not doc["response"]["ok"]:
                remove_doc_ids.add(i["doc_id"])
                continue
            else:
                try:
                    # handle the exception to bring a light on production debugging if needed.
                    r = json.loads(doc["response"]["response"])
                    ms = int(doc["response"]["ms"])
                except Exception as e:
                    remove_doc_ids.add(i["doc_id"])
                    eval_logger.error(
                        "Bad JSON data format",
                        response=doc["response"]["response"],
                        errpr=str(e),
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
                eval_logger.error(
                    "Bad JSON data format",
                    prompt_data=instance.prompt.data,
                    error=str(e),
                )
                continue
            instance.prompt.data = CompletionRequest(**request_data)

            try:
                r['response_time'] = ms
                instance.resp = CompletionResponse(**r)
            except Exception as e:
                remove_doc_ids.add(i["doc_id"])
                eval_logger.error(
                    "Bad JSON CompletionResponse format",
                    response=r,
                    error=str(e),
                )
                continue

            instances.append(instance)

        result_height = max(list_result_height)

        if len(instances) == 0 and len(remove_doc_ids) > 0:
            return [], [], result_height

        # Remove uncompleted docs_ids
        if len(remove_doc_ids) > 0:
            instances = [i for i in instances if i.doc_id not in remove_doc_ids]
            for i in instances:
                kept_doc_ids.add(i.doc_id)

        instances = sorted(instances, key=lambda x: (x.doc_id, x.idx))

        return instances, sorted(list(kept_doc_ids)), result_height

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
            times=[]
        ).model_dump(by_alias=True)

        async with self.client.start_transaction() as session:
            await self.client.db[self.tasks_collection].find_one_and_update(
                {"_id": task_id},
                {"$set": {"drop": True}},
                session=session,
            )
            await self.client.db[self.results_collection].insert_one(
                empty_result,
                session=session,
            )
