from typing import List

from temporalio import activity
from packages.python.common.auto_heartbeater import auto_heartbeater
from app.app import get_app_logger, get_app_config
from packages.python.lmeh.utils.mongodb import MongoOperator

@activity.defn
@auto_heartbeater
async def lookup_tasks() -> List[str]:
    app_config = get_app_config()
    eval_logger = get_app_logger("lookup_tasks")
    config = app_config['config']

    mongo_client = config["mongo_client"]
    mongo_operator = MongoOperator(client=mongo_client)

    eval_logger.debug(f"Searching for tasks.")

    docs = await mongo_operator.get_tasks()

    ids = [str(doc['_id']) for doc in docs]

    eval_logger.debug(f"Lookup tasks found {len(ids)} tasks")

    return ids
