from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy
from temporalio.exceptions import ApplicationError
from packages.python.protocol.protocol import PocketNetworkEvaluationTaskRequest
from app.app import get_app_logger
from activities.lmeh.evaluate import lmeh_evaluate
from activities.get_task_data import get_task_data
from activities.trigger_manager_analysis import trigger_manager_analysis
from activities.signatures.tokenizer_evaluate import tokenizer_evaluate


@workflow.defn
class Evaluator:
    @workflow.run
    async def run(self, args: PocketNetworkEvaluationTaskRequest) -> bool:
        eval_logger = get_app_logger("Evaluator")
        eval_logger.info("Starting Workflow Evaluator")
        # Extract framework and task to evaluate
        framework, task = await workflow.execute_activity(
            get_task_data,
            args,
            start_to_close_timeout=timedelta(seconds=10),
            retry_policy=RetryPolicy(maximum_attempts=2),
        )

        # Perform the corresponding evaluation
        if framework == "lmeh":
            _ = await workflow.execute_activity(
                lmeh_evaluate,
                args,
                start_to_close_timeout=timedelta(seconds=300),
                retry_policy=RetryPolicy(maximum_attempts=2),
            )

        elif framework == "signatures":
            if task == "tokenizer":
                _ = await workflow.execute_activity(
                    tokenizer_evaluate,
                    args,
                    start_to_close_timeout=timedelta(seconds=300),
                    retry_policy=RetryPolicy(maximum_attempts=2),
                )
            else:
                raise ApplicationError(
                    f"Task {task} of framework {framework} is not implemented yet.",
                    args,
                    type="BadParams",
                    non_retryable=True,
                )

        else:
            raise ApplicationError(
                f"{framework} framework not implemented yet",
                args,
                type="BadParams",
                non_retryable=True,
            )
        
        # Trigger the manager result processing workflow
        status = await workflow.execute_activity(
            trigger_manager_analysis,
            args,
            start_to_close_timeout=timedelta(seconds=10),
            retry_policy=RetryPolicy(maximum_attempts=2),
        )

        eval_logger.info("Workflow Evaluator done")
        return status
