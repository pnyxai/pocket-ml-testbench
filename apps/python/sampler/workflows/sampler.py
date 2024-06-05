from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy
from packages.python.protocol.protocol import  PocketNetworkTaskRequest
from temporalio.exceptions import ApplicationError


with workflow.unsafe.imports_passed_through():
    # add this to ensure app config is available on the thread
    from app.app import get_app_logger, get_app_config
    # add any activity that needs to be used on this workflow
    from activities.lmeh.sample import lmeh_sample as lmeh_sample
    from activities.signatures.signatures import sign_sample as sign_sample
    from pydantic import BaseModel
    from packages.python.protocol.converter import pydantic_data_converter


@workflow.defn
class Sampler:
    @workflow.run
    async def run(self, params: PocketNetworkTaskRequest) -> bool:
        if params.framework == "lmeh":
            return await workflow.execute_activity(
                lmeh_sample,
                params,
                start_to_close_timeout=timedelta(seconds=300),
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
            raise ApplicationError(f"{params.framework} framework not implemented yet")
