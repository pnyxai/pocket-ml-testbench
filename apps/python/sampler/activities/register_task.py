from temporalio import activity
from app.app import get_app_logger, get_app_config


@activity.defn
async def register_task(task_name: str) -> int:
    # this is how we get the logger, but in Temporal is recommended to avoid
    config = get_app_config()
    l = get_app_logger("register_task")
    l.info("starting activity register task")
    return 1
