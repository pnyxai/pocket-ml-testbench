from typing import List

from temporalio import activity
from packages.python.common.auto_heartbeater import auto_heartbeater
from app.app import get_app_logger, get_app_config
from packages.python.lmeh.utils.mongodb import MongoOperator


@activity.defn
@auto_heartbeater
async def get_supplier_ids() -> List[str]:
    app_config = get_app_config()
    summary_logger = get_app_logger("get_supplier_ids")
    config = app_config["config"]

    mongo_client = config["mongo_client"]
    mongo_operator = MongoOperator(client=mongo_client)

    summary_logger.debug("Searching for tasks.")
    docs = await mongo_operator.get_suppliers()
    ids = [str(doc["_id"]) for doc in docs]
    summary_logger.debug(f"Lookup suppliers found {len(ids)} suppliers")

    return ids
