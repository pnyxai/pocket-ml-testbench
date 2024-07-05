from bson import ObjectId


# Define the aggregation pipeline template
def aggregate_doc_ids(task_id: ObjectId):
    return [
        {"$match": {"task_id": task_id}},
        {"$project": {"doc_id": 1}},
        {"$unset": "_id"},
        {"$group": {"_id": None, "doc_ids": {"$addToSet": "$doc_id"}}},
        {"$unwind": "$doc_ids"},
        {
            "$sort": {
                "doc_ids": 1  # Use 1 for ascending order, -1 for descending order
            }
        },
        {"$group": {"_id": None, "doc_ids": {"$push": "$doc_ids"}}},
        {"$project": {"_id": 0, "doc_ids": 1}},
    ]


def aggregate_response_tree(task_id: ObjectId):
    return [
        {"$match": {"_id": task_id}},
        {
            "$lookup": {
                "from": "instances",
                "localField": "_id",
                "foreignField": "task_id",
                "as": "instance",
            }
        },
        {"$unwind": {"path": "$instance"}},
        {
            "$lookup": {
                "from": "prompts",
                "localField": "instance._id",
                "foreignField": "instance_id",
                "as": "prompt",
            }
        },
        {"$unwind": {"path": "$prompt"}},
        {
            "$lookup": {
                "from": "responses",
                "localField": "prompt._id",
                "foreignField": "prompt_id",
                "as": "response",
            }
        },
        {"$unwind": {"path": "$response"}},
    ]
