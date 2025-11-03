from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy
from temporalio.exceptions import ApplicationError
from packages.python.protocol.protocol import PocketNetworkSupplierSnapshotTaskRequest
from app.app import get_app_logger
from activities.supplier_snapshot import supplier_snapshot


@workflow.defn
class SuppliersSnapshot:
    @workflow.run
    async def run(self, args: PocketNetworkSupplierSnapshotTaskRequest) -> bool:
        summary_logger = get_app_logger("supplier_snapshot")
        summary_logger.info("Starting Workflow Supplier Snapshot")

        # Simply execute the taxonomy summarizer activity
        ok, msg_str = await workflow.execute_activity(
            supplier_snapshot,
            args,
            start_to_close_timeout=timedelta(seconds=600),
            retry_policy=RetryPolicy(maximum_attempts=2),
        )

        if not ok:
            raise ApplicationError(
                msg_str,
                args,
                type="SupplierSnapshotError",
                non_retryable=True,
            )

        summary_logger.info("Workflow Supplier Snapshot done")
        return True
