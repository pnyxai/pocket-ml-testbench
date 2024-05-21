from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy

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
        eval_logger = get_app_logger("Register")
        eval_logger.debug("TU MALDITA MADRE PYTHON TE ODIO - Register.run")
        wf_id = workflow.info().workflow_id
        eval_logger.debug(f"##################### Starting Workflow {wf_id} Register")
        result = False

        try:
            if args.framework == "lmeh":
                eval_logger.debug(f"##################### Calling activity lmeh_register_task - {wf_id}")
                result = await workflow.execute_activity(
                    lmeh_register_task,
                    args,
                    start_to_close_timeout=timedelta(seconds=300),
                    retry_policy=RetryPolicy(maximum_attempts=2),
                )
                eval_logger.debug(f"##################### activity lmeh_register_task done - {wf_id}")
            elif args.framework == "helm":
                # TODO: Add helm evaluation
                pass
        except Exception as e:
            eval_logger.debug(f"##################### Workflow {wf_id} Register run in error", e)
            return result

        eval_logger.debug(f"##################### Workflow {wf_id} Register done")
        return result
