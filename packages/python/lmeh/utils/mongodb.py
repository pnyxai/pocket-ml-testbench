import json
import os
import shutil
import pymongo

from dataclasses import asdict
from bson.objectid import ObjectId
from lm_eval.api.instance import Instance
from transformers import PreTrainedTokenizer, PreTrainedTokenizerFast
from typing import Union, List
from pathlib import Path
from temporalio.exceptions import ApplicationError
from packages.python.protocol.protocol import  PocketNetworkMongoDBTask, CompletionRequest, PocketNetworkMongoDBPrompt, CompletionResponse
from packages.python.lmeh.utils.mongo_aggrs import agrr_doc_ids, agrr_response_tree

from app.app import get_app_logger
eval_logger = get_app_logger("sample")
evaluation_logger = get_app_logger("evaluation")


def reconstruct_instance(_id: ObjectId, collection: pymongo.collection.Collection):
    """
    Reconstructs an Instance object from a MongoDB document.

    Args:
        _id (str): The ID of the document to reconstruct.
        collection (pymongo.collection.Collection): The MongoDB collection to query.

    Returns:
        Instance: The reconstructed Instance object.
    """

    instance = collection.find_one({"_id": _id})
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

def get_tokenizer_objects(
        adress: str, service: str, client: pymongo.MongoClient, db_name:str='pocket-ml-testbench', 
        nodes_collection_name:str='nodes', tokenizers_collection_name:str='tokenizers'
        )-> dict:
    
    node = list(client[db_name][nodes_collection_name].find({'address': adress, 'service': service}))
    eval_logger.debug(f"Node found.", node=node)
    if len(node) == 0:
        eval_logger.error(f"Node adress not found.", adress=adress)
        raise ApplicationError(f"Node adress {adress} does not exist in the database.")    
    elif len(node) > 1:
        eval_logger.error(f"Multiple nodes found for adress.", adress=adress)
        raise ApplicationError(f"Multiple nodes found for adress {adress}.")
    else:
        node = node[0]

    tokenizer_objects = list(client[db_name][tokenizers_collection_name].find({'hash': node['tokenizer']}))
    # Validate that the tokenizer is not empty
    if len(tokenizer_objects) == 0:
        eval_logger.error(f"Tokenizer hash not found.", adress=adress, hash=node['tokenizer'])
        raise ApplicationError(f"Tokenizer with hash {node['tokenizer']} does not exist in the database.")
    elif len(tokenizer_objects) > 1:
        eval_logger.error(f"Multiple tokenizers found for hash.", adress=adress, hash=node['tokenizer'])
        raise ApplicationError(f"Multiple tokenizers found for hash {node['tokenizer']}.")
    else:
        tokenizer_objects = tokenizer_objects[0]['tokenizer']
    eval_logger.debug(f"Tokenizer found.", tokenizer_keys=list(tokenizer_objects.keys()))

    if 'model_max_length' in tokenizer_objects['tokenizer_config']:
        tokenizer_objects['tokenizer_config']['model_max_length'] = int(tokenizer_objects['tokenizer_config']['model_max_length'])

    return tokenizer_objects

def get_prompt_request(request_id: ObjectId, client: pymongo.MongoClient, db_name:str='pocket-ml-testbench',
                collection='prompts')->CompletionRequest:
    prompt_doc = list(client[db_name][collection].find({'_id': request_id}))
    if len(prompt_doc) == 0:
        eval_logger.error(f"Prompt request not found.", request_id=request_id)
        raise ApplicationError(f"Prompt request with ID {request_id} does not exist in the database.")
    elif len(prompt_doc) > 1:
        eval_logger.error(f"Multiple prompt requests found for ID.", request_id=request_id)
        raise ApplicationError(f"Multiple prompt requests found for ID {request_id}.")
    else:
        data = prompt_doc[0]['data']
        data = json.loads(data)
        request = CompletionRequest(**data)
    eval_logger.debug(f"Prompt request found.", request_id=request_id)
    return request

###############################################
# Evaluator
################################################

def get_doc_ids_by_task(task_id: ObjectId, client: pymongo.MongoClient, db_name:str='pocket-ml-testbench',
                collection='instances')->List[int]:
    # Create the aggregation pipeline with the given task_id
    aggr = agrr_doc_ids(task_id)
    # Execute the aggregation
    result = list(client[db_name][collection].aggregate(aggr))
    if len(result) == 0:
        evaluation_logger.error(f"Task ID not found.", task_id=task_id)
        raise ApplicationError(f"Task ID {task_id} does not exist in the database.")
    # Convert the result to a list and return it
    doc_ids = result[0]['doc_ids']
    return doc_ids

def get_task(task_id: ObjectId, client: pymongo.MongoClient, db_name:str='pocket-ml-testbench',
                collection='tasks'):
    task = list(client[db_name][collection].find({'_id': task_id}))
    if len(task) == 0:
        evaluation_logger.error(f"Task ID not found.", task_id=task_id)
        raise ApplicationError(f"Task ID {task_id} does not exist in the database.")
    elif len(task) > 1:
        evaluation_logger.error(f"Multiple tasks found for ID.", task_id=task_id)
        raise ApplicationError(f"Multiple tasks found for ID {task_id}.")
    else:
        task = task[0]
    task.pop('_id', None)
    evaluation_logger.debug(f"task:", task=task)
    task = PocketNetworkMongoDBTask(**task)
    task.id = task_id
    return task

def reconstruct_instances(task_id: ObjectId, client: pymongo.MongoClient, db_name:str='pocket-ml-testbench',
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