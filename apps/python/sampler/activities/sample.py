from temporalio import activity
from app.app import get_app_logger, get_app_config


@activity.defn
async def sample(params: dict) -> int:
    # this is how we get the logger, but in Temporal is recommended to avoid
    config = get_app_config()
    # config["postgres"]
    # config["mongodb"]
    l = get_app_logger("sample")
    l.info("starting activity sample", params)
    return 1
