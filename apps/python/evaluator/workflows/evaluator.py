from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy
from temporalio.exceptions import ApplicationError
from packages.python.protocol.protocol import  PocketNetworkEvaluationTaskRequest
with workflow.unsafe.imports_passed_through():
    # add this to ensure app config is available on the thread
    from app.app import get_app_logger, get_app_config
    # add any activity that needs to be used on this workflow
    from activities.lmeh.evaluate import evaluation as lmeh_evaluation
    from pydantic import BaseModel
    from packages.python.protocol.converter import pydantic_data_converter


@workflow.defn
class Evaluator:
    @workflow.run
    async def run(self, args: PocketNetworkEvaluationTaskRequest) -> bool:
#        if args.framework == "lmeh":
        _ = await workflow.execute_activity(
            lmeh_evaluation,
            args,
            start_to_close_timeout=timedelta(seconds=300),
            retry_policy=RetryPolicy(maximum_attempts=2),
        )
        return True
        # raise ApplicationError(f"{args.framework} framework not implemented yet")
