from typing import Tuple
from datetime import datetime
from temporalio import activity
from packages.python.common.auto_heartbeater import auto_heartbeater
from app.app import get_app_logger, get_app_config
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.protocol.protocol import PocketNetworkTaxonomySummaryTaskRequest
from packages.python.protocol.protocol import PocketNetworkMongoDBTaxonomySummary
from packages.python.protocol.protocol import TaxonomyNodeSummary
import numpy as np
from temporalio.exceptions import ApplicationError
from bson import ObjectId


@activity.defn
@auto_heartbeater
async def summarize_taxonomy(
    args: PocketNetworkTaxonomySummaryTaskRequest,
) -> Tuple[bool, str]:
    app_config = get_app_config()
    summary_logger = get_app_logger("summarize_taxonomy")
    config = app_config["config"]
    taxonomies = app_config["taxonomies"]

    mongo_client = config["mongo_client"]
    mongo_operator = MongoOperator(client=mongo_client)

    # Get this taxonomy
    taxonomy_graph = taxonomies.get(args.taxonomy, None)
    if taxonomy_graph is None:
        return (
            False,
            f'Requested taxonomy "{args.taxonomy}" not found in configuration.',
        )

    # Create base result
    result = PocketNetworkMongoDBTaxonomySummary(
        supplier_id=ObjectId(args.supplier_id),
        summary_date=datetime.today().isoformat(),
        taxonomy_name=args.taxonomy,
        taxonomy_nodes_scores=dict(),
    )

    # Fill with taxonomy nodes
    for node in taxonomy_graph.nodes:
        running_score_total = 0
        running_score_square_dev = 0
        running_time_total = 0
        running_time_square_dev = 0
        runnning_n = 0
        sample_min = np.inf

        if node == "root_c":
            continue
        for dataset in taxonomy_graph.nodes[node]["datasets"]:
            # Get data for this node and dataset
            framework = "lmeh"  # TODO : Remove hardcode
            try:
                docs = await mongo_operator.get_supplier_results_for_task(
                    ObjectId(args.supplier_id), framework, dataset
                )
                if len(docs) > 1:
                    return (
                        False,
                        f"Found multiple buffers ({len(docs)}) for supplier {args.supplier_id}, in framework {framework} and task {dataset}.",
                    )
            except Exception as e:
                return False, str(e)

            # No data, continue
            if len(docs) == 0:
                summary_logger.warn(
                    "No results found for supplier.",
                    supplier_id=args.supplier_id,
                    framework=framework,
                    task=dataset,
                )
                continue

            # Data
            this_result = docs[0]

            # Get number of samples here
            samples_here = int(this_result["samples"] * (1 - this_result["error_rate"]))
            if samples_here == 0:
                # No valid samples to calculate
                continue
            # Track minimum in this node (used for later check of sample coverage)
            if sample_min > samples_here:
                sample_min = samples_here
            # Calculate the partial score
            running_score_total += this_result["mean_scores"]
            mean_dev_here = this_result["std_scores"] / np.sqrt(samples_here)
            running_score_square_dev += mean_dev_here**2
            # Calculate the partial times
            running_time_total += this_result["mean_times"]
            mean_dev_here = this_result["std_times"] / np.sqrt(samples_here)
            running_time_square_dev += mean_dev_here**2
            # Track mean samples
            runnning_n += 1

        # Fill node metrics
        if runnning_n > 0:
            result.taxonomy_nodes_scores[node] = TaxonomyNodeSummary(
                score=running_score_total / runnning_n,
                score_dev=np.sqrt(running_score_square_dev),
                run_time=running_time_total / runnning_n,
                run_time_dev=np.sqrt(running_time_square_dev),
                sample_min=sample_min,
            )
        else:
            result.taxonomy_nodes_scores[node] = TaxonomyNodeSummary(
                score=0, score_dev=0, run_time=0, run_time_dev=0, sample_min=0
            )

    # Calculate root (grand average)
    running_score_total = 0
    running_score_square_dev = 0
    running_time_total = 0
    running_time_square_dev = 0
    runnning_n = 0
    sample_min = np.inf
    for edge in taxonomy_graph.edges("root_c"):
        assert "root_c" == edge[0]  # Otherwise the taxonomy is malformed

        running_score_total += result.taxonomy_nodes_scores[edge[1]].score
        running_score_square_dev += result.taxonomy_nodes_scores[edge[1]].score_dev ** 2

        running_time_total += result.taxonomy_nodes_scores[edge[1]].run_time
        running_time_square_dev += (
            result.taxonomy_nodes_scores[edge[1]].run_time_dev ** 2
        )

        runnning_n += 1
        if sample_min > result.taxonomy_nodes_scores[edge[1]].sample_min:
            sample_min = result.taxonomy_nodes_scores[edge[1]].sample_min

    result.taxonomy_nodes_scores["root_c"] = TaxonomyNodeSummary(
        score=running_score_total / runnning_n,
        score_dev=np.sqrt(running_score_square_dev),
        run_time=running_time_total / runnning_n,
        run_time_dev=np.sqrt(running_time_square_dev),
        sample_min=sample_min,
    )

    # Save result to mongo
    try:
        async with mongo_client.start_transaction() as session:
            try:
                result_dump = result.model_dump(by_alias=True)
                result_dump.pop("_id", None)  # We cannot replace the id
                await mongo_client.db[
                    mongo_operator.taxonomy_summaries
                ].find_one_and_replace(
                    {"supplier_id": ObjectId(args.supplier_id), "taxonomy_name": args.taxonomy},
                    result_dump,
                    upsert=True,
                    return_document=False,
                    session=session,
                )
            except Exception as e:
                summary_logger.error(
                    "Unable to save taxonomy summary.",
                    task_id=id,
                    error=str(e),
                )
                raise ApplicationError(
                    "Unable to save taxonomy summary.",
                    str(e),
                    type="Mongodb",
                    non_retryable=True,
                )

    except Exception as e:
        summary_logger.error(
            "Failed to setup MongoDB session (taxonomy summary).", error=e
        )
        raise ApplicationError(
            "Failed to setup MongoDB session (taxonomy summary).",
            str(e),
            type="Mongodb",
            non_retryable=True,
        )

    summary_logger.debug(
        f"Success summary for {args.supplier_id} in taxonomy {args.taxonomy}"
    )

    return True, ""
