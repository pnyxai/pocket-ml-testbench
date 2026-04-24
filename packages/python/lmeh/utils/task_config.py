import json
import os

TESTBENCH_TASK_CONFIG_FILE = os.getenv("TESTBENCH_TASK_CONFIG_FILE", None)

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
    "t-eval_pnyx_instruct-v2": {
        "metrics": ["call_match"],
        "filters": ["instruct_extract"],
    },
    "t-eval_pnyx_plan-json-v2": {
        "metrics": ["a-plan_f1_score"],
        "filters": ["call_extract"],
    },
    "t-eval_pnyx_plan-reason-retrieve-understand-json-v2": {
        "metrics": ["call_match"],
        "filters": ["call_extract"],
    },
    "t-eval_pnyx_review-str-v2": {
        "metrics": ["a-vert_match"],
        "filters": ["pass_all"],
    },
    "debugbench_python_": {
        "metrics": ["pass@1"],
        "filters": ["create_test"],
    },
}

if TESTBENCH_TASK_CONFIG_FILE is not None:
    print(f"Overriding task config using file: {TESTBENCH_TASK_CONFIG_FILE}")
    with open(TESTBENCH_TASK_CONFIG_FILE) as f:
        task_cnfg = json.load(f)

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
    elif "debugbench_python_" in task_name:
        return task_cnfg["debugbench_python_"]

    else:
        retun_config =  task_cnfg.get(task_name, None)
        if retun_config is None:
            raise ValueError(f"Task {task_name} is not defined, please pass a custom TESTBENCH_TASK_CONFIG_FILE json.")
        return retun_config
