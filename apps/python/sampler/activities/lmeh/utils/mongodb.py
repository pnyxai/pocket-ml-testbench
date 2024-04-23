import pymongo
from bson.objectid import ObjectId
from lm_eval.api.instance import Instance

def reconstruct_Instance(_id: str, collection: pymongo.collection.Collection):
    """
    Reconstructs an Instance object from a MongoDB document.

    Args:
        _id (str): The ID of the document to reconstruct.
        collection (pymongo.collection.Collection): The MongoDB collection to query.

    Returns:
        Instance: The reconstructed Instance object.
    """

    instance = collection.find_one({"_id": ObjectId(_id)})
    valid_fields = {field.name for field in Instance.__dataclass_fields__.values()}
    instance_dict = {key: value for key, value in instance.items() if key in valid_fields}
    instance = Instance(**instance_dict)

    # TODO 
    # 1) GET PROMPT RESPONSE
    
    # 2) PUT RESPONSE IN `Instance.resp` like in:
    #       for x, req in zip(resps, cloned_reqs):
    #           req.resps.append(x)
    
    return instance