task_cnfg = {
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
        "filters": ["flexible-extract"]
        }
}

def get_task_config(task_name:str):
    if "mmlu" in task_name:
        return task_cnfg["mmlu"]
    return task_cnfg[task_name]