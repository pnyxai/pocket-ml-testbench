from temporalio import activity
from temporalio import workflow
from temporalio.exceptions import ApplicationError
from packages.python.common.auto_heartbeater import auto_heartbeater
from app.app import get_app_logger, get_app_config
from packages.python.lmeh.utils.mongodb import MongoOperator

# Custom modules
from packages.python.protocol.protocol import PocketNetworkEvaluationTaskRequest
from bson import ObjectId


@activity.defn
@auto_heartbeater
async def trigger_manager_analysis(args: PocketNetworkEvaluationTaskRequest) -> bool:
    app_config = get_app_config()
    eval_logger = get_app_logger("evaluation")
    config = app_config["config"]

    args.task_id

    workflow.start_child_workflow(
            config["temporal"]["manager-result-analyzer"]["workflow_name"],
            id="result-analysis-%s"%str(args.task_id),
            args=[{"task_id": args.task_id}],
            task_queue=config["temporal"]["manager-result-analyzer"]["task_queue"]
        )


    return True
