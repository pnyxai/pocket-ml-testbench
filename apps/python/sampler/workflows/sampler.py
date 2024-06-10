from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy
from packages.python.protocol.protocol import  PocketNetworkTaskRequest
from temporalio.exceptions import ApplicationError

from app.app import get_app_logger
from activities.lmeh.sample import lmeh_sample
from activities.signatures.signatures import sign_sample


@workflow.defn
class Sampler:
    @workflow.run
    async def run(self, params: PocketNetworkTaskRequest) -> bool:
        eval_logger = get_app_logger("Sampler")
        eval_logger.info("Starting Workflow Sampler")

        if params.framework == "lmeh":
            result = await workflow.execute_activity(
                lmeh_sample,
                params,
                start_to_close_timeout=timedelta(seconds=300),
                heartbeat_timeout=timedelta(seconds=60),
                retry_policy=RetryPolicy(maximum_attempts=2),
            )
        elif params.framework == "signatures":
            return await workflow.execute_activity(
                sign_sample,
                params,
                start_to_close_timeout=timedelta(seconds=300),
                retry_policy=RetryPolicy(maximum_attempts=2),
            )
        else:
            raise ApplicationError(
                f"{params.framework} framework not implemented yet",
                params,
                type="BadParams",
                non_retryable=True
            )

        eval_logger.info("Workflow Sampler done")
        return result
