from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy
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
        eval_logger = get_app_logger("Sampler")
        eval_logger.debug("TU MALDITA MADRE PYTHON TE ODIO - Sampler.run")

        wf_id = workflow.info().workflow_id
        eval_logger.debug(f"##################### Starting Workflow {wf_id} Sampler")
        result = False
        try:
            if params.framework == "lmeh":
                eval_logger.debug(f"##################### Calling activity lmeh_sample - {wf_id}")
                await workflow.execute_local_activity(
                    lmeh_sample,
                    params,
                    start_to_close_timeout=timedelta(seconds=120),
                    retry_policy=RetryPolicy(maximum_attempts=1),
                )
                eval_logger.debug(f"##################### activity lmeh_sample done - {wf_id}")
            elif params.framework == "helm":
                # TODO: Add helm evaluation
                pass
        except Exception as e:
            eval_logger.debug(f"##################### Workflow {wf_id} Sampler run in error", e)
            return result

        eval_logger.debug(f"##################### Workflow {wf_id} Sampler done")
        return result
