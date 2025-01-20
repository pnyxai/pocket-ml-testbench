task_cnfg = {   
    # Uses: Legacy HF leaderboard
    "arc_challenge": { 
        "metrics": ["acc_norm"],
        "num_fewshot": 25,
    },
    "hellaswag": {
        "metrics": ["acc_norm"],
        "num_fewshot": 10,
    },
    "truthfulqa_mc2": {
        "metrics": ["acc"],
        "num_fewshot": 0,
    },
    "mmlu": {
        "metrics": ["acc"],
        "num_fewshot": 5,
    },
    "winogrande": {
        "metrics": ["acc"],
        "num_fewshot": 5,
    },
    "gsm8k": {
        "metrics": ["exact_match"],
        "num_fewshot": 5,
        "filters": ["flexible-extract"],
    },

    # Uses: HF leaderboard
    "gpqa_main_generative_n_shot": {
        "metrics": ["exact_match"],
        "num_fewshot": 0,
        "filters": ["flexible-extract"],
    },
    "leaderboard_musr": {
        "metrics": ["acc_norm"],
        "num_fewshot": 0,
    },
    "leaderboard_math_hard": {
        "metrics": ["exact_match"],
        "num_fewshot": 4,
        "filters": ["strict-match", "flexible-extract"],
    },
    # Uses: HF leaderboard / Taxonomy [Alpha]
    # "bbh_fewshot_fix": {
    #     "metrics": ["exact_match"],
    #     "num_fewshot": 3,
    #     "filters": ["remove_whitespace"],
    # },
    "mmlu_pro": {
        "metrics": ["exact_match"],
        "num_fewshot": 3,
        "filters": ["custom-extract", "get_response"],
    },
    "ifeval": {
        "metrics": ["prompt_level_strict_acc", "inst_level_strict_acc", "prompt_level_loose_acc", "inst_level_loose_acc"],
        "num_fewshot": 0,
    },
    # Uses: Taxonomy [Alpha]
    "babi": {
        "metrics": ["exact_match"],
        "num_fewshot": 3,
        "filters": ["get_response"],
    },
    "mmlu_generative": {
        "metrics": ["exact_match"],
        "num_fewshot": 3,
        "filters": ["get_response"],
    },
    # "humaneval": {
    #     "metrics": ["!function utils.pass_at_1"],
    #     "num_fewshot": 0,
    # },

}


def get_task_config(task_name: str):
    if "mmlu" in task_name:
        if "generative" in task_name:
            return task_cnfg["mmlu_generative"]
        elif "pro" in task_name:
            return task_cnfg["mmlu_pro"]    
        else:
            return task_cnfg["mmlu"]
    if "babi" in task_name:
        return task_cnfg["babi"]
    return task_cnfg[task_name]
