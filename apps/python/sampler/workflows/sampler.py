from datetime import timedelta
from temporalio import workflow
from protocol.protocol import PocketNetworkTaskRequest, PocketNetworkRegisterTaskRequest
with workflow.unsafe.imports_passed_through():
    # add this to ensure app config is available on the thread
    from app.app import get_app_logger, get_app_config
    # add any activity that need to be used on this workflow
    from activities.lmeh.register_task import register_task as lmeh_register_task
    from activities.lmeh.sample import sample as lmeh_sample
    from pydantic import BaseModel
    from protocol.converter import pydantic_data_converter

@workflow.defn
class Sampler:
    @workflow.run
    async def run(self, params: PocketNetworkTaskRequest) -> bool:
        if params.framework == "lmeh":
            await workflow.execute_activity(
                lmeh_register_task,
                params,
                schedule_to_close_timeout=timedelta(seconds=30),
            )

            await workflow.execute_activity(
                lmeh_sample,
                params,
                schedule_to_close_timeout=timedelta(seconds=30),
            )
        elif params.framework == "helm":
            # TODO: Add helm evaluation
            pass

        return True
