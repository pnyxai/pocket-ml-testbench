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
    config = app_config["config"]

    mongo_client = config["mongo_client"]
    mongo_operator = MongoOperator(client=mongo_client)

    eval_logger.debug("Searching for tasks.")
    docs = await mongo_operator.get_tasks()
    ids = [str(doc["_id"]) for doc in docs]
    eval_logger.debug(f"Lookup tasks found {len(ids)} done tasks")

    # Look for skipped/old tasks
    eval_logger.debug("Searching for old/skipped tasks.")
    docs = await mongo_operator.get_old_tasks(blocks_ago=40)
    old_ids = [str(doc["_id"]) for doc in docs]
    eval_logger.debug(f"Lookup tasks found {len(old_ids)} old/skipped tasks")
    # Set them as done
    eval_logger.debug("Setting skipped tasks as done.")
    old_ids_ok = list()
    for id in old_ids:
        try:
            mongo_operator.set_task_as_done(id)
            old_ids_ok.append(id)
        except Exception as e:
            eval_logger.error(
                "Unable to mark task as done. If this persist, the task will stay in the database and prevent further task triggers.",
                task_id=id,
                error=str(e),
            )

    eval_logger.debug(f"Lookup tasks found {len(ids + old_ids_ok)} old/skipped tasks")

    return ids + old_ids_ok
