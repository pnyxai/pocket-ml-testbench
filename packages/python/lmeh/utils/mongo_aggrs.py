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


def aggregate_old_tasks(latest_height: int, blocks_ago: int):
    return [
        {"$match": {"done": False}},
        {"$project": {"total_instances": 1}},
        {
            "$lookup": {
                "from": "responses",
                "localField": "_id",
                "foreignField": "task_id",
                "as": "responses",
            }
        },
        {
            "$project": {
                "_id": 1,
                "total": "$total_instances",
                "done": {"$size": "$responses"},
                "last_seen": {"$max": "$responses.height"},
            }
        },
        {
            "$match": {
                "$expr": {
                    "$and": [
                        {"$gte": ["$done", "$total"]},
                        {"$lte": ["$last_seen", latest_height - blocks_ago]},
                    ]
                }
            }
        },
        {"$project": {"_id": 1}},
    ]


# Define the aggregation pipeline template
def aggregate_supplier_task_results(supplier_id: ObjectId, framework: str, task: str):
    return [
        {
            "$match": {
                "task_data.supplier_id": supplier_id,
                "task_data.framework": framework,
                "task_data.task": task,
            }
        },
        {
            "$project": {
                "mean_scores": 1,
                "mean_times": 1,
                "median_scores": 1,
                "median_times": 1,
                "std_scores": 1,
                "std_times": 1,
                "samples": "$circ_buffer_control.num_samples",
                "error_rate": 1,
            }
        },
    ]
