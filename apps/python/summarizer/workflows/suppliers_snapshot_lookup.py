from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy, WorkflowIDReusePolicy
from app.app import get_app_logger, get_app_config
from packages.python.common.utils import get_from_dict
from workflows.suppliers_snapshot import SuppliersSnapshot
from activities.get_supplier_ids import get_supplier_ids
from temporalio.workflow import ParentClosePolicy


@workflow.defn
class SuppliersSnapshotLookup:
    @workflow.run
    async def run(self) -> int:
        app_config = get_app_config()
        summary_logger = get_app_logger("suppliers_snapsho_lookup")
        config = app_config["config"]
        task_queue = get_from_dict(config, "temporal.task_queue")
        summary_logger.info("Starting Workflow Suppliers Snapshot Lookup")

        # Get suppliers ids to test
        ids = await workflow.execute_activity(
            get_supplier_ids,
            start_to_close_timeout=timedelta(seconds=60),
            retry_policy=RetryPolicy(maximum_attempts=1),
        )
        summary_logger.info(f'Activity "get_supplier_ids" found {len(ids)} suppliers')

        # For each supplier, trigger a snapshot workflow
        for _id in ids:
            summary_logger.debug(f"Triggering Snapshot workflow for supplier {_id}")
            try:
                await workflow.start_child_workflow(
                    SuppliersSnapshot,
                    {
                        "supplier_id": _id,
                    },
                    id=f"snapshot-{_id}",
                    task_queue=task_queue,
                    execution_timeout=timedelta(seconds=600),
                    task_timeout=timedelta(seconds=600),
                    id_reuse_policy=WorkflowIDReusePolicy.ALLOW_DUPLICATE,
                    retry_policy=RetryPolicy(maximum_attempts=1),
                    parent_close_policy=ParentClosePolicy.ABANDON,
                )
            except Exception as e:
                summary_logger.warn(
                    f'Unable to trigger workflow for task "snapshot-{_id}": {e}'
                )
                pass

        summary_logger.info("Workflow Supplier Snapshot Lookup done")
        return len(ids)
