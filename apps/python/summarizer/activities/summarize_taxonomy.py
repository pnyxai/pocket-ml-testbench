from typing import Tuple
from datetime import datetime
from math import sqrt

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

            # Scores. A dataset's standard error is std / sqrt(n), with n - 1
            # degrees of freedom.
            score_acc.add(
                score.get("mean"),
                score.get("median"),
                float(score.get("std") or 0.0) / sqrt(n_score),
                n_score,
                n_score - 1,
            )

            # Times
            n_time = int(time.get("n") or 0)
            if n_time > 0:
                time_acc.add(
                    time.get("mean"),
                    time.get("median"),
                    float(time.get("std") or 0.0) / sqrt(n_time),
                    n_time,
                    n_time - 1,
                )

            # Cost (per position, skipping not-informed values). Each position
            # has its own standard error and degrees of freedom.
            cost_mean = cost.get("mean") or []
            cost_median = cost.get("median") or []
            cost_std = cost.get("std") or []
            cost_n = cost.get("n") or []
            cost_len = max(len(cost_mean), len(cost_median), len(cost_std), len(cost_n))
            cost_se = []
            cost_df = []
            for position in range(cost_len):
                n_pos = cost_n[position] if position < len(cost_n) else 0
                std_pos = cost_std[position] if position < len(cost_std) else None
                if n_pos and n_pos > 0 and std_pos is not None:
                    cost_se.append(float(std_pos) / sqrt(n_pos))
                    cost_df.append(n_pos - 1)
                else:
                    cost_se.append(None)
                    cost_df.append(0)
            cost_acc.add(cost_mean, cost_median, cost_se, cost_df)

        # Fill node metrics
        if score_acc.n() > 0:
            result.taxonomy_nodes_scores[node] = TaxonomyNodeSummary(
                stats=TaskStats(
                    score=ScalarStats(
                        mean=score_acc.mean(),
                        median=score_acc.median(),
                        se=score_acc.se(),
                        v_eff=score_acc.v_eff(),
                    ),
                    time=ScalarStats(
                        mean=time_acc.mean(),
                        median=time_acc.median(),
                        se=time_acc.se(),
                        v_eff=time_acc.v_eff(),
                    ),
                    cost=VectorStats(
                        mean=cost_acc.mean(),
                        median=cost_acc.median(),
                        se=cost_acc.se(),
                        v_eff=cost_acc.v_eff(),
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

    # Calculate root (grand average) from its direct children. Children without
    # any data (sample_min == 0) do not contribute to the mean or its error.
    score_acc = ScalarStatsAccumulator()
    time_acc = ScalarStatsAccumulator()
    cost_acc = VectorStatsAccumulator()
    for edge in taxonomy_graph.edges("root_c"):
        assert "root_c" == edge[0]  # Otherwise the taxonomy is malformed
        child = result.taxonomy_nodes_scores[edge[1]]
        if child.sample_min == 0:
            continue
        node_stats = child.stats

        score_acc.add(
            node_stats.score.mean,
            node_stats.score.median,
            node_stats.score.se,
            child.sample_min,
            node_stats.score.v_eff,
        )
        time_acc.add(
            node_stats.time.mean,
            node_stats.time.median,
            node_stats.time.se,
            child.sample_min,
            node_stats.time.v_eff,
        )
        cost_acc.add(
            node_stats.cost.mean,
            node_stats.cost.median,
            node_stats.cost.se,
            node_stats.cost.v_eff,
        )

    result.taxonomy_nodes_scores["root_c"] = TaxonomyNodeSummary(
        stats=TaskStats(
            score=ScalarStats(
                mean=score_acc.mean(),
                median=score_acc.median(),
                se=score_acc.se(),
                v_eff=score_acc.v_eff(),
            ),
            time=ScalarStats(
                mean=time_acc.mean(),
                median=time_acc.median(),
                se=time_acc.se(),
                v_eff=time_acc.v_eff(),
            ),
            cost=VectorStats(
                mean=cost_acc.mean(),
                median=cost_acc.median(),
                se=cost_acc.se(),
                v_eff=cost_acc.v_eff(),
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
