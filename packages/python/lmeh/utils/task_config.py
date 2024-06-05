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


#ARC: 25-shot, arc-challenge (acc_norm)
#HellaSwag: 10-shot, hellaswag (acc_norm)
#TruthfulQA: 0-shot, truthfulqa-mc (mc2)
#MMLU: 5-shot, hendrycksTest-abstract_algebra,hendrycksTest-anatomy,hendrycksTest-astronomy,hendrycksTest-business_ethics,hendrycksTest-clinical_knowledge,hendrycksTest-college_biology,hendrycksTest-college_chemistry,hendrycksTest-college_computer_science,hendrycksTest-college_mathematics,hendrycksTest-college_medicine,hendrycksTest-college_physics,hendrycksTest-computer_security,hendrycksTest-conceptual_physics,hendrycksTest-econometrics,hendrycksTest-electrical_engineering,hendrycksTest-elementary_mathematics,hendrycksTest-formal_logic,hendrycksTest-global_facts,hendrycksTest-high_school_biology,hendrycksTest-high_school_chemistry,hendrycksTest-high_school_computer_science,hendrycksTest-high_school_european_history,hendrycksTest-high_school_geography,hendrycksTest-high_school_government_and_politics,hendrycksTest-high_school_macroeconomics,hendrycksTest-high_school_mathematics,hendrycksTest-high_school_microeconomics,hendrycksTest-high_school_physics,hendrycksTest-high_school_psychology,hendrycksTest-high_school_statistics,hendrycksTest-high_school_us_history,hendrycksTest-high_school_world_history,hendrycksTest-human_aging,hendrycksTest-human_sexuality,hendrycksTest-international_law,hendrycksTest-jurisprudence,hendrycksTest-logical_fallacies,hendrycksTest-machine_learning,hendrycksTest-management,hendrycksTest-marketing,hendrycksTest-medical_genetics,hendrycksTest-miscellaneous,hendrycksTest-moral_disputes,hendrycksTest-moral_scenarios,hendrycksTest-nutrition,hendrycksTest-philosophy,hendrycksTest-prehistory,hendrycksTest-professional_accounting,hendrycksTest-professional_law,hendrycksTest-professional_medicine,hendrycksTest-professional_psychology,hendrycksTest-public_relations,hendrycksTest-security_studies,hendrycksTest-sociology,hendrycksTest-us_foreign_policy,hendrycksTest-virology,hendrycksTest-world_religions (average of all the results acc)
#Winogrande: 5-shot, winogrande (acc)
#GSM8k: 5-shot, gsm8k (acc)