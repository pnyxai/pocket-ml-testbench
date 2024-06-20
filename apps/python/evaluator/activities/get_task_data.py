from temporalio import activity
from temporalio.exceptions import ApplicationError
from packages.python.common.auto_heartbeater import auto_heartbeater
from app.app import get_app_logger, get_app_config
from packages.python.lmeh.utils.mongodb import MongoOperator

# Custom modules
from packages.python.protocol.protocol import PocketNetworkEvaluationTaskRequest
from bson import ObjectId


@activity.defn
@auto_heartbeater
async def get_task_data(args: PocketNetworkEvaluationTaskRequest) -> tuple[str, str]:
    app_config = get_app_config()
    eval_logger = get_app_logger("evaluation")
    config = app_config['config']

    mongo_client = config["mongo_client"]
    mongo_operator = MongoOperator(client=mongo_client)

    eval_logger.debug(f"Searching for task {args.task_id}.")

    try:
        args.task_id = ObjectId(args.task_id)
    except Exception as e:
        raise ApplicationError(
            "Bad Task ID format",
            str(e), args.task_id,
            type="BadParams",
            non_retryable=True,
        )

    task_mongo = await mongo_operator.get_task(args.task_id)

    eval_logger.debug(f"Found! Evaluating [{task_mongo.framework}][{task_mongo.tasks}].")

    return task_mongo.framework, task_mongo.tasks
