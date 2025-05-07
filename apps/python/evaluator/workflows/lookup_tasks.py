from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy, WorkflowIDReusePolicy
from app.app import get_app_logger, get_app_config
from packages.python.common.utils import get_from_dict
from activities.lookup_tasks import lookup_tasks
from workflows.evaluator import Evaluator
from temporalio.workflow import ParentClosePolicy


@workflow.defn
class LookupTasks:
    @workflow.run
    async def run(self) -> int:
        app_config = get_app_config()
        eval_logger = get_app_logger("lookup_tasks")
        config = app_config["config"]
        task_queue = get_from_dict(config, "temporal.task_queue")
        eval_logger.info("Starting Workflow LookupTasks")
        # Extract framework and task to evaluate
        ids = await workflow.execute_activity(
            lookup_tasks,
            start_to_close_timeout=timedelta(seconds=60),
            retry_policy=RetryPolicy(maximum_attempts=1),
        )

        eval_logger.info(f"Activity Lookup tasks found {len(ids)} tasks")
            

        for _id in ids:
            eval_logger.info(f"Triggering Evaluate workflow for task {_id}")
            try:
                await workflow.start_child_workflow(
                    Evaluator,
                    {"task_id": _id},
                    id=_id,
                    task_queue=task_queue,
                    execution_timeout=timedelta(seconds=120),
                    task_timeout=timedelta(seconds=60),
                    id_reuse_policy=WorkflowIDReusePolicy.ALLOW_DUPLICATE_FAILED_ONLY,
                    retry_policy=RetryPolicy(maximum_attempts=1),
                    parent_close_policy=ParentClosePolicy.ABANDON,
                )
            except Exception as e:
                eval_logger.warn(f"Unable to trigger workflow for task {_id}: {e}")
                pass

        eval_logger.info("Workflow LookupTasks done")
        return len(ids)
