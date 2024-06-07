task_cnfg = {
    "arc_challenge": {
        "metric": "acc_norm",
        "num_fewshot":25
        },
    "hellaswag": {
        "metric": "acc_norm",
        "num_fewshot":10
        },
    "truthfulqa_mc2": {
        "metric": "mc2",
        "num_fewshot":0
        },
    "mmlu": {
        "metric": "average",
        "num_fewshot":5
        },
    "winogrande": {
        "metric": "acc",
        "num_fewshot":5
        },
    "gsm8k": {
        "metric": "acc",
        "num_fewshot":5
        }
}

def get_task_config(task_name:str):
    if "mmlu" in task_name:
        return task_cnfg["mmlu"]
    return task_cnfg[task_name]