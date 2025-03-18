from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy, WorkflowIDReusePolicy
from app.app import get_app_logger, get_app_config
from packages.python.common.utils import get_from_dict
from workflows.taxonomy_summary import TaxonomySummarizer
from activities.get_node_ids import get_node_ids
from temporalio.workflow import ParentClosePolicy


@workflow.defn
class TaxonomySummaryLookup:
    @workflow.run
    async def run(self) -> int:
        app_config = get_app_config()
        summary_logger = get_app_logger("summarize_taxonomy")
        config = app_config["config"]
        task_queue = get_from_dict(config, "temporal.task_queue")
        summary_logger.info("Starting Workflow Taxonomy Summary Lookup")

        # Retrieve list of taxonomies to analyze
        taxonomies = list(app_config["taxonomies"].keys())
        summary_logger.info(f"Analyzing {len(taxonomies)} taxonomies")

        # Get nodes ids to test
        ids = await workflow.execute_activity(
            get_node_ids,
            start_to_close_timeout=timedelta(seconds=60),
            retry_policy=RetryPolicy(maximum_attempts=1),
        )
        summary_logger.info(f'Activity "get_node_ids" found {len(ids)} nodes')

        # For each node and taxonomy, trigger a summary workflow
        for _id in ids:
            for tax in taxonomies:
                summary_logger.debug(
                    f"Triggering Summary workflow for taxonomy {tax} and node {_id}"
                )
                try:
                    await workflow.start_child_workflow(
                        TaxonomySummarizer,
                        {
                            "node_id": _id,
                            "taxonomy": tax,
                        },
                        id=f"{tax}-{_id}",
                        task_queue=task_queue,
                        execution_timeout=timedelta(seconds=600),
                        task_timeout=timedelta(seconds=600),
                        id_reuse_policy=WorkflowIDReusePolicy.ALLOW_DUPLICATE,
                        retry_policy=RetryPolicy(maximum_attempts=1),
                        parent_close_policy=ParentClosePolicy.ABANDON,
                    )
                except Exception as e:
                    summary_logger.warn(
                        f'Unable to trigger workflow for task "{tax}-{_id}": {e}'
                    )
                    pass

        summary_logger.info("Workflow Taxonomy Summary Lookup done")
        return len(ids)
