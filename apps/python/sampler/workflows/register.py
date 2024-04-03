from datetime import timedelta
from temporalio import workflow

with workflow.unsafe.imports_passed_through():
    # add this to ensure app config is available on the thread
    from app.app import get_app_logger
    # add any activity that need to be used on this workflow
    from activities.lookup_task import lookup_task
    from activities.register_task import register_task


@workflow.defn
class Register:
    @workflow.run
    async def run(self, task_name: str) -> int:
        # print("x")
        l = get_app_logger("register")
        # print("y", l)
        l.info("starting workflow registration with")
        # print("z")

        x = await workflow.execute_activity(
            lookup_task,
            task_name,
            schedule_to_close_timeout=timedelta(seconds=5),
        )

        y = await workflow.execute_activity(
            register_task,
            task_name,
            schedule_to_close_timeout=timedelta(seconds=5),
        )

        result = x + y
        l.info("result", result=result)
        return result
