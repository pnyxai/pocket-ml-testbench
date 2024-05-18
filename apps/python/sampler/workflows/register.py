from datetime import timedelta
from temporalio import workflow

with workflow.unsafe.imports_passed_through():
    # add this to ensure app config is available on the thread
    from app.app import get_app_logger
    # add any activity that need to be used on this workflow
    from activities.lmeh.register_task import register_task as lmeh_register_task
    from protocol.protocol import PocketNetworkRegisterTaskRequest
    from pydantic import BaseModel
    from protocol.converter import pydantic_data_converter    


@workflow.defn
class Register:
    @workflow.run
    async def run(self, args: PocketNetworkRegisterTaskRequest) -> bool:
        if args.framework == "lmeh":
            x = await workflow.execute_activity(
                lmeh_register_task,
                args,
                start_to_close_timeout=timedelta(seconds=120),
            )
        elif args.framework == "helm":
            # TODO: Add helm evaluation
            pass
        return x
