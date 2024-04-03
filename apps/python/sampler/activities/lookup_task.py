from temporalio import activity
from app.app import get_app_logger, get_app_config


@activity.defn
async def lookup_task(task_name: str) -> int:
    # this is how we get the logger, but in Temporal is recommended to avoid
    config = get_app_config()
    l = get_app_logger("lookup_task")
    l.info("starting activity lookup task")
    return 1
