import json
from typing import List
from motor.motor_asyncio import AsyncIOMotorClient, AsyncIOMotorCollection
from dataclasses import asdict
from bson.objectid import ObjectId
from lm_eval.api.instance import Instance
from temporalio.exceptions import ApplicationError
from packages.python.protocol.protocol import  PocketNetworkMongoDBTask, CompletionRequest, PocketNetworkMongoDBPrompt, CompletionResponse
from packages.python.lmeh.utils.mongo_aggrs import agrr_doc_ids, agrr_response_tree

from app.app import get_app_logger
from packages.python.common.mongodb import MongoClient
eval_logger = get_app_logger("sample")
evaluation_logger = get_app_logger("evaluation")


async def reconstruct_instance(_id: str, collection: AsyncIOMotorCollection):
    """
    Reconstructs an Instance object from a MongoDB document.

    Args:
        _id (str): The ID of the document to reconstruct.
        collection (pymongo.collection.Collection): The MongoDB collection to query.

    Returns:
        Instance: The reconstructed Instance object.
    """

    instance = await collection.find_one({"_id": ObjectId(_id)})
    if instance is None:
        raise ApplicationError(f"Instance {_id} does not exist in the database.")

    valid_fields = {field.name for field in Instance.__dataclass_fields__.values()}
    instance_dict = {key: value for key, value in instance.items() if key in valid_fields}
    instance = Instance(**instance_dict)

    # TODO 
    # 1) GET PROMPT RESPONSE

    # 2) PUT RESPONSE IN `Instance.resp` like in:
    #       for x, req in zip(resps, cloned_reqs):
    #           req.resps.append(x)

    return instance

# TODO : This should reffer to PocketNetworkMongoDBInstance and not depend on LMEH blindly
def instance_to_dict(instance: Instance, task_id: ObjectId)-> dict:
    instance_mongo = asdict(instance)
    instance_mongo.pop('resps', None)
    instance_mongo.pop('filtered_resps', None)
    instance_mongo['task_id'] = task_id
    instance_mongo['_id'] = ObjectId()
    instance_mongo['done'] = False
    return instance_mongo

async def get_tokenizer_objects(
        address: str, service: str,
        client: MongoClient,
        nodes_collection_name: str = 'nodes',
        tokenizers_collection_name: str = 'tokenizers'
) -> dict:
    node = await client.db[nodes_collection_name].find_one({'address': address, 'service': service})

    if node is None:
        eval_logger.error("Node address not found.", adress=address)
        raise ApplicationError(f"Node address {address} does not exist in the database.")

    eval_logger.debug("Node found.", node=node)

    # Check if tokenizer signature exists
    if node.get('signature_tasks', None) == None:
        eval_logger.error("Node address has no signature_tasks, cannot load tokenizer hash.", adress=address)
        raise ApplicationError(f"Node address {address}, has no signature_tasks cannot load tokenizer hash.")

    tokenizer_hash = ''
    for task in node['signature_tasks']:
        if (task['task_data']['framework'] == 'signatures') and (task['task_data']['task'] == 'tokenizer'):
            tokenizer_hash = task['last_signature']
    if tokenizer_hash == '':
        eval_logger.error("Node address does not have a valid tokenizer_hash.", adress=address)
        raise ApplicationError(f"Node address {address} does not have a valid tokenizer_hash.")

    tokenizer_object = await client.db[tokenizers_collection_name].find_one({'hash': tokenizer_hash})

    # Validate that the tokenizer is not empty
    if tokenizer_object is None:
        eval_logger.error(f"Tokenizer hash not found.", address=address, hash=tokenizer_hash)
        raise ApplicationError(f"Tokenizer with hash {tokenizer_hash} does not exist in the database.")

    tokenizer = tokenizer_object['tokenizer']
    eval_logger.debug("Tokenizer found.", tokenizer_keys=list(tokenizer.keys()))

    if 'model_max_length' in tokenizer['tokenizer_config']:
        tokenizer['tokenizer_config']['model_max_length'] = int(
            tokenizer['tokenizer_config']['model_max_length'])

    return tokenizer

async def get_prompt_request(
        request_id: ObjectId,
        client: AsyncIOMotorClient,
        collection='prompts',
) -> CompletionRequest:
    prompt_doc = await client.db[collection].find_one({'_id': request_id})

    if prompt_doc is None:
        eval_logger.error("Prompt request not found.", request_id=request_id)
        raise ApplicationError(f"Prompt request with ID {request_id} does not exist in the database.")

    data = prompt_doc['data']
    data = json.loads(data)
    request = CompletionRequest(**data)
    eval_logger.debug(f"Prompt request found.", request_id=request_id)

    return request

###############################################
# Evaluator
################################################

async def get_doc_ids_by_task(task_id: ObjectId, client: MongoClient,
                collection='instances')->List[int]:
    # Create the aggregation pipeline with the given task_id
    aggr = agrr_doc_ids(task_id)
    # Execute the aggregation
    result = await client.db[collection].aggregate(aggr)
    if len(result) == 0:
        evaluation_logger.error(f"Task ID not found.", task_id=task_id)
        raise ApplicationError(f"Task ID {task_id} does not exist in the database.")
    # Convert the result to a list and return it
    doc_ids = result[0]['doc_ids']
    return doc_ids

async def get_task(task_id: ObjectId, client: MongoClient,
                collection='tasks'):
    task = await client.db[collection].find_one({'_id':  task_id})
    if task is None:
        evaluation_logger.error(f"Task ID not found.", task_id=task_id)
        raise ApplicationError(f"Task ID {task_id} does not exist in the database.")
    task.pop('_id', None)
    evaluation_logger.debug(f"task:", task=task)
    task = PocketNetworkMongoDBTask(**task)
    task.id = task_id
    return task

def reconstruct_instances(task_id: ObjectId, client: MongoClient, db_name:str='pocket-ml-testbench',
                collection='tasks')->List[Instance]:
    result = list(client[db_name][collection].aggregate(agrr_response_tree(task_id)))
    if len(result) == 0:
        evaluation_logger.error(f"Task ID not found.", task_id=task_id)
        raise ApplicationError(f"Task ID {task_id} does not exist in the database.")
    valid_fields = {field.name for field in Instance.__dataclass_fields__.values()}
    instances = []
    for doc in result:
        i, p, r = doc['instance'], doc['prompt'], json.loads(doc['response']['response'])
        instance_dict = {key: value for key, value in i.items() if key in valid_fields}
        instance = Instance(**instance_dict)
        instance.repeats = 1 # to avoid double evaluation for each instance
        instance.prompt = PocketNetworkMongoDBPrompt(**p)
        instance.prompt.data = CompletionRequest(**json.loads(instance.prompt.data))
        instance.resp = CompletionResponse(**r)
        instances.append(instance)
    instances = sorted(instances, key=lambda x: (x.doc_id, x.idx))
    return instances