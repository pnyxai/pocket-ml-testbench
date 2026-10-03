from typing import Tuple
from datetime import datetime

from pymongo import ReturnDocument
from temporalio import activity
from packages.python.common.auto_heartbeater import auto_heartbeater
from app.app import get_app_logger, get_app_config
from packages.python.lmeh.utils.cost_stats import (
    ScalarStatsAccumulator,
    VectorStatsAccumulator,
)
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.protocol.protocol import PocketNetworkTaxonomySummaryTaskRequest
from packages.python.protocol.protocol import PocketNetworkMongoDBTaxonomySummary
from packages.python.protocol.protocol import ScalarStats, TaskStats, VectorStats
from packages.python.protocol.protocol import TaxonomyNodeSummary
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
    valid_node_data = False
    for node in taxonomy_graph.nodes:
        score_acc = ScalarStatsAccumulator()
        time_acc = ScalarStatsAccumulator()
        cost_acc = VectorStatsAccumulator()
        if node == "root_c":
            continue
        for dataset in taxonomy_graph.nodes[node]["datasets"]:
            # Get data for this node and dataset
            framework_subs = "lmeh"  # TODO : Remove hardcode, aggregation is matching any framework that contain this
            try:
                docs = await mongo_operator.get_supplier_results_for_task(
                    ObjectId(args.supplier_id), framework_subs, dataset
                )
                if len(docs) > 1:
                    return (
                        False,
                        f"Found multiple buffers ({len(docs)}) for supplier {args.supplier_id}, with framework substring {framework_subs} and task {dataset}.",
                    )
            except Exception as e:
                return False, str(e)

            # No data, continue
            if len(docs) == 0:
                # summary_logger.warn(
                #     "No results found for supplier.",
                #     supplier_id=args.supplier_id,
                #     framework=framework_subs,
                #     task=dataset,
                # )
                continue

            # Data
            this_result = docs[0]
            stats = this_result.get("stats") or {}
            score = stats.get("score") or {}
            time = stats.get("time") or {}
            cost = stats.get("cost") or {}

            # Number of valid samples backing the scalar statistics. No valid
            # samples means there is nothing to aggregate for this dataset.
            n_score = int(score.get("n") or 0)
            if n_score == 0:
                continue

            # Scores (the sample count is adjusted via the per-field `n`).
            score_acc.add_mean(score.get("mean"))
            score_acc.add_median(score.get("median"))
            score_acc.add_sem(score.get("std"), n_score)
            score_acc.add_min(n_score)

            # Times
            n_time = int(time.get("n") or 0)
            time_acc.add_mean(time.get("mean"))
            time_acc.add_median(time.get("median"))
            time_acc.add_sem(time.get("std"), n_time)
            time_acc.add_min(n_time)

            # Cost (per position, skipping not-informed values). Each position
            # uses its own sample count for the standard error.
            cost_acc.add_mean(cost.get("mean") or [])
            cost_acc.add_median(cost.get("median") or [])
            cost_acc.add_sem(cost.get("std") or [], cost.get("n") or [])
            cost_acc.add_min(cost.get("n") or [])

        # Fill node metrics
        if score_acc.n() > 0:
            result.taxonomy_nodes_scores[node] = TaxonomyNodeSummary(
                stats=TaskStats(
                    score=ScalarStats(
                        mean=score_acc.mean(),
                        median=score_acc.median(),
                        std=score_acc.std(),
                        n=score_acc.n(),
                    ),
                    time=ScalarStats(
                        mean=time_acc.mean(),
                        median=time_acc.median(),
                        std=time_acc.std(),
                        n=time_acc.n(),
                    ),
                    cost=VectorStats(
                        mean=cost_acc.mean(),
                        median=cost_acc.median(),
                        std=cost_acc.std(),
                        n=cost_acc.n(),
                    ),
                ),
                sample_min=score_acc.n(),
            )
            valid_node_data = True
        else:
            result.taxonomy_nodes_scores[node] = TaxonomyNodeSummary()

    if not valid_node_data:
        summary_logger.debug(
            f"No data to process summary for {args.supplier_id} in taxonomy {args.taxonomy}"
        )
        return True, "No data to summarize"

    # Calculate root (grand average)
    score_acc = ScalarStatsAccumulator()
    time_acc = ScalarStatsAccumulator()
    cost_acc = VectorStatsAccumulator()
    for edge in taxonomy_graph.edges("root_c"):
        assert "root_c" == edge[0]  # Otherwise the taxonomy is malformed
        node_stats = result.taxonomy_nodes_scores[edge[1]].stats

        score_acc.add_mean(node_stats.score.mean)
        score_acc.add_median(node_stats.score.median)
        score_acc.add_sem(node_stats.score.std, 1)
        score_acc.add_min(node_stats.score.n)

        time_acc.add_mean(node_stats.time.mean)
        time_acc.add_median(node_stats.time.median)
        time_acc.add_sem(node_stats.time.std, 1)
        time_acc.add_min(node_stats.time.n)

        # Combine child costs element-wise (n=1 mirrors the score deviation
        # aggregation above, i.e. sqrt(sum(child_std ** 2))).
        cost_acc.add_mean(node_stats.cost.mean)
        cost_acc.add_median(node_stats.cost.median)
        cost_acc.add_sem(node_stats.cost.std, 1)
        cost_acc.add_min(node_stats.cost.n)

    result.taxonomy_nodes_scores["root_c"] = TaxonomyNodeSummary(
        stats=TaskStats(
            score=ScalarStats(
                mean=score_acc.mean(),
                median=score_acc.median(),
                std=score_acc.std(),
                n=score_acc.n(),
            ),
            time=ScalarStats(
                mean=time_acc.mean(),
                median=time_acc.median(),
                std=time_acc.std(),
                n=time_acc.n(),
            ),
            cost=VectorStats(
                mean=cost_acc.mean(),
                median=cost_acc.median(),
                std=cost_acc.std(),
                n=cost_acc.n(),
            ),
        ),
        sample_min=score_acc.n(),
    )

    # Save result to mongo
    try:
        async with mongo_client.start_transaction() as session:
            try:
                result_dump = result.model_dump(by_alias=True)
                result_dump.pop("_id", None)  # We cannot replace the id
                _ = await mongo_client.db[
                    mongo_operator.taxonomy_summaries
                ].find_one_and_replace(
                    {
                        "supplier_id": ObjectId(args.supplier_id),
                        "taxonomy_name": args.taxonomy,
                    },
                    result_dump,
                    upsert=True,
                    return_document=ReturnDocument.BEFORE,
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
