from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy
from temporalio.exceptions import ApplicationError
from packages.python.protocol.protocol import PocketNetworkTaxonomySummaryTaskRequest
from app.app import get_app_logger
from activities.summarize_taxonomy import summarize_taxonomy


@workflow.defn
class TaxonomySummarizer:
    @workflow.run
    async def run(self, args: PocketNetworkTaxonomySummaryTaskRequest) -> bool:
        summary_logger = get_app_logger("summarize_taxonomy")
        summary_logger.info("Starting Workflow Taxonomy Summary")

        # Simply execute the taxonomy summarizer activity
        ok, msg_str = await workflow.execute_activity(
            summarize_taxonomy,
            args,
            start_to_close_timeout=timedelta(seconds=600),
            retry_policy=RetryPolicy(maximum_attempts=2),
        )

        if not ok:
            raise ApplicationError(
                msg_str,
                args,
                type="TaxonomySummarizeError",
                non_retryable=True,
            )

        summary_logger.info("Workflow Taxonomy Summarizer done")
        return True
