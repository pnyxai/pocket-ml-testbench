from app.basemodels import (
    PoktMongodb,
    agg_get_nodes_ids,
    agg_data_node,
    agg_data_scores,
)
from bson import ObjectId
from copy import deepcopy
import numpy as np
import pandas as pd
from typing import Tuple
from app.logger import init_logger

logger = init_logger(__name__)

LEADERBOARD_FRAMEWORK = "lmeh"
LEADERBOARD_METRICS = {
    "arc_challenge": "arc",
    "hellaswag": "hellaswag",
    "mmlu": "mmlu",
    "truthfulqa_mc2": "truthfulqa",
    "winogrande": "winogrande",
    "gsm8k": "gsm8k",
}
LEADERBOARD_MMLU_METRICS = [
    "mmlu_abstract_algebra",
    "mmlu_anatomy",
    "mmlu_astronomy",
    "mmlu_business_ethics",
    "mmlu_clinical_knowledge",
    "mmlu_college_biology",
    "mmlu_college_chemistry",
    "mmlu_college_computer_science",
    "mmlu_college_mathematics",
    "mmlu_college_medicine",
    "mmlu_college_physics",
    "mmlu_computer_security",
    "mmlu_conceptual_physics",
    "mmlu_econometrics",
    "mmlu_electrical_engineering",
    "mmlu_elementary_mathematics",
    "mmlu_formal_logic",
    "mmlu_global_facts",
    "mmlu_high_school_biology",
    "mmlu_high_school_chemistry",
    "mmlu_high_school_computer_science",
    "mmlu_high_school_european_history",
    "mmlu_high_school_geography",
    "mmlu_high_school_government_and_politics",
    "mmlu_high_school_macroeconomics",
    "mmlu_high_school_mathematics",
    "mmlu_high_school_microeconomics",
    "mmlu_high_school_physics",
    "mmlu_high_school_psychology",
    "mmlu_high_school_statistics",
    "mmlu_high_school_us_history",
    "mmlu_high_school_world_history",
    "mmlu_human_aging",
    "mmlu_human_sexuality",
    "mmlu_international_law",
    "mmlu_jurisprudence",
    "mmlu_logical_fallacies",
    "mmlu_machine_learning",
    "mmlu_management",
    "mmlu_marketing",
    "mmlu_medical_genetics",
    "mmlu_miscellaneous",
    "mmlu_moral_disputes",
    "mmlu_moral_scenarios",
    "mmlu_nutrition",
    "mmlu_philosophy",
    "mmlu_prehistory",
    "mmlu_professional_accounting",
    "mmlu_professional_law",
    "mmlu_professional_medicine",
    "mmlu_professional_psychology",
    "mmlu_public_relations",
    "mmlu_security_studies",
    "mmlu_sociology",
    "mmlu_us_foreign_policy",
    "mmlu_virology",
    "mmlu_world_religions",
]


async def get_all_nodes_ids(mongodb):
    aggr_use = deepcopy(agg_get_nodes_ids)
    result = await mongodb.query("nodes", aggr_use)
    return list(result)


async def get_leaderboard_full(mongodb: PoktMongodb) -> Tuple[dict, bool]:
    success = False

    id_list = await get_all_nodes_ids(mongodb)

    leaderboard = dict()

    for entry in id_list:
        try:
            # Get node data
            aggr_use = deepcopy(agg_data_node)
            aggr_use[0]["$match"]["_id"] = ObjectId(entry["_id"])
            result = await mongodb.query("nodes", aggr_use)
            list_cur = list(result)

            node_df = pd.DataFrame(list_cur)

            # Get scores data
            aggr_use = deepcopy(agg_data_scores)
            aggr_use[0]["$match"]["task_data.node_id"] = ObjectId(entry["_id"])
            result = await mongodb.query("buffers_numerical", aggr_use)
            list_cur = list(result)

            scores_df = pd.DataFrame(list_cur)

            # Prepare entry
            leaderboard_entry = dict()

            # Add QoS
            leaderboard_entry["qos"] = {
                "error_rate": (np.random.random(1)[0] * 0.1),
                "response_time": (np.random.random(1)[0] * 1000),
            }

            # Add Metrics
            leaderboard_entry["metrics"] = dict()
            running_mean_avg = 0
            weight_avg = 0
            std_err_avg = 0
            for metric in LEADERBOARD_METRICS.keys():
                metric_name = LEADERBOARD_METRICS[metric]

                if metric == "mmlu":
                    running_mean_mmlu = 0
                    weight_mmlu = 0
                    std_err_mmlu = 0
                    # This requiere more steps yay!
                    for mmlu_metric in LEADERBOARD_MMLU_METRICS:
                        data_row = scores_df.loc[
                            (scores_df["framework"] == LEADERBOARD_FRAMEWORK)
                            * (scores_df["task"] == mmlu_metric)
                        ]
                        assert len(data_row) == 1
                        if data_row["num"].values[0] > 0:
                            metric_mean = (
                                data_row["mean"].values[0] * data_row["num"].values[0]
                            )
                            metric_std_err = data_row["std"].values[0] / np.sqrt(
                                data_row["num"].values[0]
                            )
                        else:
                            metric_mean = 0
                            metric_std_err = 0

                        this_w = data_row["num"].values[0]
                        running_mean_mmlu += metric_mean
                        weight_mmlu += this_w
                        if this_w > 0:
                            std_err_mmlu += (metric_std_err / this_w) ** 2

                    if weight_mmlu == 0:
                        running_mean_mmlu = 0
                    else:
                        running_mean_mmlu = running_mean_mmlu / weight_mmlu
                        if std_err_mmlu != 0:
                            std_err_mmlu = np.sqrt(std_err_mmlu)

                    metric_mean = running_mean_mmlu
                    metric_std_err = std_err_mmlu
                    metric_weight = weight_mmlu / len(LEADERBOARD_MMLU_METRICS)

                else:
                    data_row = scores_df.loc[
                        (scores_df["framework"] == LEADERBOARD_FRAMEWORK)
                        * (scores_df["task"] == metric)
                    ]
                    assert len(data_row) == 1

                    if data_row["num"].values[0] > 0:
                        metric_mean = data_row["mean"].values[0]
                        metric_std_err = data_row["std"].values[0] / np.sqrt(
                            data_row["num"].values[0]
                        )
                        metric_weight = data_row["num"].values[0]
                    else:
                        metric_mean = 0
                        metric_std_err = 0
                        metric_weight = 0

                leaderboard_entry["metrics"][metric_name] = {
                    "mean": metric_mean,
                    "stderr": metric_std_err,
                }

                running_mean_avg += metric_mean * metric_weight
                weight_avg += metric_weight
                std_err_avg += metric_std_err**2

            if weight_avg == 0:
                running_mean_avg = 0
            else:
                running_mean_avg = running_mean_avg / weight_avg
                if std_err_avg != 0:
                    std_err_avg = np.sqrt(std_err_avg)

            leaderboard_entry["metrics"]["average"] = {
                "mean": running_mean_avg,
                "stderr": std_err_avg,
            }

            # Add Metadata
            leaderboard_entry["metadata"] = {
                "service": str(node_df["service"].values[0]),
                "last_seen_height": int(node_df["last_seen_height"].values[0]),
                "last_seen_time": node_df["last_seen_time"].values[0].astype(str),
            }

            # Add to leaderboard
            address = node_df["address"].values[0]
            leaderboard[address] = leaderboard_entry

        except Exception as e:
            print(str(e))
            logger.warn(
                f"Failed to retrieve leaderboard data for node : {entry} error: {str(e)}"
            )

    if len(leaderboard) > 0:
        success = True
    return leaderboard, success
