task_cnfg = {
    # Uses: Taxonomy [Alpha]
    "gsm8k_chat": {
        "metrics": ["a-vert_match"],
        "filters": ["pass_all"],
    },
    "gpqa_subtask": {
        "metrics": ["a-vert_match"],
        "filters": ["pass_all"],
    },
    "ifeval": {
        "metrics": [
            # "prompt_level_strict_acc",
            # "inst_level_strict_acc",
            "prompt_level_loose_acc",
            # "inst_level_loose_acc",
        ],
        "num_fewshot": 0,
    },
    "bbh-split_": {
        "metrics": ["a-vert_match"],
        "filters": ["pass_all"],
    },
    "mmlu_pro-category_": {
        "metrics": ["a-vert_match"],
        "filters": ["pass_all"],
    },
    "babisteps-chat_zero_shot": {
        "metrics": ["a-vert_match"],
        "filters": ["pass_all"],
    },
    "mmlu_chat_generative": {
        "metrics": ["a-vert_match"],
        "filters": ["pass_all"],
    },
    # "humaneval": {
    #     "metrics": ["!function utils.pass_at_1"],
    #     "num_fewshot": 0,
    # },
    "reasoning_gym": {
        "metrics": ["score_match"],
        "filters": ["pass_all"],
    },
}


def get_task_config(task_name: str):
    if "mmlu" in task_name:
        if "chat_generative" in task_name:
            return task_cnfg["mmlu_chat_generative"]
        else:
            return task_cnfg["mmlu_pro-category_"]

    elif "babisteps" in task_name:
        return task_cnfg["babisteps-chat_zero_shot"]
    elif "bbh-split_" in task_name:
        return task_cnfg["bbh-split_"]
    elif "gpqa_subtask" in task_name:
        return task_cnfg["gpqa_subtask"]
    elif "reasoning_gym" in task_name:
        return task_cnfg["reasoning_gym"]

    else:
        return task_cnfg[task_name]
