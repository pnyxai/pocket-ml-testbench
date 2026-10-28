from datetime import datetime

from app.app import get_app_config, get_app_logger
from bson import ObjectId
from temporalio import activity
from temporalio.exceptions import ApplicationError

from packages.python.common.auto_heartbeater import auto_heartbeater
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.lmeh.utils.stats_compat import (
    is_legacy_buffer_stats,
    is_legacy_taxonomy_node,
)
from packages.python.lmeh.utils.stats_schema import (
    StatsSchemaError,
    parse_raw_task_stats,
    parse_taxonomy_node,
)
from packages.python.protocol.protocol import (
    NumericSampleSnapshot,
    PocketNetworkMongoDBSupplierSnapshot,
    PocketNetworkSupplierSnapshotTaskRequest,
)


@activity.defn
@auto_heartbeater
async def supplier_snapshot(
    args: PocketNetworkSupplierSnapshotTaskRequest,
) -> tuple[bool, str]:
    app_config = get_app_config()
    summary_logger = get_app_logger("supplier_snapshot")
    config = app_config["config"]
    mongo_client = config["mongo_client"]
    mongo_operator = MongoOperator(client=mongo_client)

    # Get all tasks data
    summary_logger.debug("retrieving supplier tasks (supplier_snapshot)")
    try:
        task_docs = await mongo_operator.get_supplier_snapshot_task_data(
            ObjectId(args.supplier_id)
        )
    except Exception as e:
        return False, str(e)
    # Convert to structure
    tasks_dict = dict()
    for doc in task_docs:
        if is_legacy_buffer_stats(doc):
            return (
                False,
                (
                    f"Legacy buffer schema detected for supplier {args.supplier_id} / "
                    f"task {doc['task']}. "
                    "Run the PR #245 migration before creating snapshots."
                ),
            )
        context = "supplier {} task {} buffer {}".format(
            args.supplier_id,
            doc.get("task"),
            doc.get("_id"),
        )
        try:
            raw_stats = parse_raw_task_stats(doc.get("stats"), context)
        except StatsSchemaError as exc:
            return False, str(exc)
        tasks_dict[doc["task"]] = NumericSampleSnapshot(
            error_rate=doc["error_rate"],
            num_samples=doc["num_samples"],
            # Keep the raw buffer statistics verbatim so taxonomy summaries can
            # be recomputed offline.
            stats=raw_stats,
        )

    # Get all taxonomy data
    summary_logger.debug("retrieving supplier taxonomies (supplier_snapshot)")
    try:
        taxonomy_docs = await mongo_operator.get_supplier_snapshot_taxonomy_data(
            ObjectId(args.supplier_id)
        )
    except Exception as e:
        return False, str(e)
    # Convert to structure
    taxonomy_dict = dict()
    for doc in taxonomy_docs:
        taxonomy_dict[doc["taxonomy_name"]] = dict()
        for node in doc["taxonomy_nodes_scores"].keys():
            node_data = doc["taxonomy_nodes_scores"][node]
            if is_legacy_taxonomy_node(node_data):
                return (
                    False,
                    "Legacy taxonomy summary schema detected for supplier {} / "
                    "taxonomy {}. Run the PR #245 migration before creating "
                    "snapshots.".format(args.supplier_id, doc["taxonomy_name"]),
                )
            context = "supplier {} taxonomy {} node {} summary {}".format(
                args.supplier_id,
                doc["taxonomy_name"],
                node,
                doc.get("_id"),
            )
            try:
                taxonomy_dict[doc["taxonomy_name"]][node] = parse_taxonomy_node(
                    node_data, context
                )
            except StatsSchemaError as exc:
                return False, str(exc)

    # Create snapshot entry
    result = PocketNetworkMongoDBSupplierSnapshot(
        supplier_id=ObjectId(args.supplier_id),
        summary_date=datetime.today().isoformat(),
        taxonomies=taxonomy_dict,
        tasks=tasks_dict,
    )

    # Save result to mongo
    summary_logger.debug("Saving snapshot to mongo (supplier_snapshot)")
    try:
        async with mongo_client.start_transaction() as session:
            try:
                result_dump = result.model_dump(by_alias=True)
                result_dump.pop("_id", None)  # We cannot replace the id
                await mongo_client.db[mongo_operator.suppliers_snapshots].insert_one(
                    result_dump,
                    session=session,
                )
            except Exception as e:
                summary_logger.error(
                    "Unable to save supplier snapshot.",
                    task_id=id,
                    error=str(e),
                )
                raise ApplicationError(
                    "Unable to save supplier snapshot.",
                    str(e),
                    type="Mongodb",
                    non_retryable=True,
                )

    except Exception as e:
        summary_logger.error(
            "Failed to setup MongoDB session (supplier snapshot).", error=e
        )
        raise ApplicationError(
            "Failed to setup MongoDB session (supplier snapshot).",
            str(e),
            type="Mongodb",
            non_retryable=True,
        )

    summary_logger.debug(f"Successful snapshot for {args.supplier_id}.")

    return True, ""
