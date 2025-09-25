from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy
from temporalio.exceptions import ApplicationError
from packages.python.protocol.protocol import PocketNetworkTaxonomySummaryTaskRequest
from app.app import get_app_logger
from activities.summarize_identity import summarize_identity


@workflow.defn
class IdentitySummarizer:
    @workflow.run
    async def run(self) -> bool:
        summary_logger = get_app_logger("summarize_identity")
        summary_logger.info("Starting Workflow Identity Summary")

        # Simply execute the identity summarizer activity
        ok, msg_str = await workflow.execute_activity(
            summarize_identity,
            start_to_close_timeout=timedelta(seconds=600),
            retry_policy=RetryPolicy(maximum_attempts=2),
        )

        if not ok:
            raise ApplicationError(
                msg_str,
                None,
                type="SummarizeError",
                non_retryable=True,
            )

        summary_logger.info("Workflow Identity Summarizer done")
        return True
