from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy
from typing import List
from protocol.protocol import CumSumRequest


with workflow.unsafe.imports_passed_through():
    # add this to ensure app config is available on the thread
    from app.app import get_app_logger, get_app_config
    # add any activity that need to be used on this workflow
    from activities.cumm_sum import random_int
    from pydantic import BaseModel


@workflow.defn
class RandomInt:
    @workflow.run
    async def run(self,a:dict) -> int:

        eval_logger = get_app_logger("Cumm_Summ")
        wf_id = workflow.info().workflow_id
        eval_logger.debug(f"Input:,",a=a)
        input=a['a']
        r1 = await workflow.execute_activity(
            random_int,
            input,
            start_to_close_timeout=timedelta(seconds=300),
            retry_policy=RetryPolicy(maximum_attempts=2),
        )
        eval_logger.debug(f"## activity lmeh_register_task done - {wf_id}")

        return r1
 