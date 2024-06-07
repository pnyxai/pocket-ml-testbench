from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy
from temporalio.exceptions import ApplicationError

from app.app import get_app_logger
from packages.python.protocol.protocol import PocketNetworkRegisterTaskRequest
from activities.lmeh.register_task import register_task as lmeh_register_task


@workflow.defn()
class Register:
    @workflow.run
    async def run(self, args: PocketNetworkRegisterTaskRequest) -> bool:
        eval_logger = get_app_logger("Register")
        eval_logger.info("Starting Workflow Register")

        if args.framework == "lmeh":
            result = await workflow.execute_activity(
                lmeh_register_task,
                args,
                start_to_close_timeout=timedelta(seconds=3600),
                heartbeat_timeout=timedelta(seconds=60),
                retry_policy=RetryPolicy(maximum_attempts=2),
            )
        else:
            raise ApplicationError(
                f"Unsupported framework {args.framework}",
                args,
                type="BadParams",
                non_retryable=True,
            )

        eval_logger.info("Workflow Register done")
        return result
