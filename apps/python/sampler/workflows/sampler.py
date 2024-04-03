from datetime import timedelta
from temporalio import workflow

with workflow.unsafe.imports_passed_through():
    # add this to ensure app config is available on the thread
    from app.app import get_app_logger
    # add any activity that need to be used on this workflow
    from activities.sample import sample


@workflow.defn
class Sampler:
    @workflow.run
    async def run(self, params: dict) -> int:
        x = await workflow.execute_activity(
            sample,
            params,
            schedule_to_close_timeout=timedelta(seconds=5),
        )

        return 0
