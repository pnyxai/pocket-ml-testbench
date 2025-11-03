from typing import Tuple
from datetime import datetime

from temporalio import activity
from packages.python.common.auto_heartbeater import auto_heartbeater
from app.app import get_app_logger, get_app_config
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.protocol.protocol import PocketNetworkSupplierSnapshotTaskRequest
from packages.python.protocol.protocol import PocketNetworkMongoDBSupplierSnapshot
from packages.python.protocol.protocol import NumericSampleSnapshot, TaxonomyNodeSummary
from temporalio.exceptions import ApplicationError
from bson import ObjectId


@activity.defn
@auto_heartbeater
async def supplier_snapshot(
    args: PocketNetworkSupplierSnapshotTaskRequest,
) -> Tuple[bool, str]:
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
        tasks_dict[doc["task"]] = NumericSampleSnapshot(
            error_rate=doc["error_rate"],
            mean_scores=doc["mean_scores"],
            mean_times=doc["mean_times"],
            median_scores=doc["median_scores"],
            median_times=doc["median_times"],
            std_scores=doc["std_scores"],
            std_times=doc["std_times"],
            num_samples=doc["num_samples"],
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
            taxonomy_dict[doc["taxonomy_name"]][node] = TaxonomyNodeSummary(
                score=doc["taxonomy_nodes_scores"][node]["score"],
                score_dev=doc["taxonomy_nodes_scores"][node]["score_dev"],
                run_time=doc["taxonomy_nodes_scores"][node]["run_time"],
                run_time_dev=doc["taxonomy_nodes_scores"][node]["run_time_dev"],
                sample_min=doc["taxonomy_nodes_scores"][node]["sample_min"],
            )

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
