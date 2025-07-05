import subprocess
import time
import argparse
import json

TEMPORAL_NAMESPACE = "pocket-ml-testbench"
APPS_PER_SERVICE = {
    "lm": ["pokt1wkra80yv9zv69y2rgkmc69jfqph6053dwn47vx"],
}

BASE_COMMAND = ["kubectl", "exec", "-it", "deploy/temporal-admintools"]

LMEH_TYPE = "lmeh"

all_taxonomy_tasks = [
    "mmlu_anatomy_generative",
    "mmlu_medical_genetics_generative",
    "mmlu_human_aging_generative",
    "mmlu_nutrition_generative",
    "mmlu_human_sexuality_generative",
    "mmlu_sociology_generative",
    "mmlu_clinical_knowledge_generative",
    "mmlu_professional_psychology_generative",
    "mmlu_professional_medicine_generative",
    "mmlu_public_relations_generative",
    "mmlu_marketing_generative",
    "mmlu_management_generative",
    "mmlu_jurisprudence_generative",
    "mmlu_professional_law_generative",
    "mmlu_high_school_government_and_politics_generative",
    "mmlu_professional_accounting_generative",
    "mmlu_us_foreign_policy_generative",
    "mmlu_philosophy_generative",
    "mmlu_world_religions_generative",
    "mmlu_econometrics_generative",
    "mmlu_global_facts_generative",
    "mmlu_high_school_geography_generative",
    "mmlu_high_school_statistics_generative",
    "mmlu_high_school_us_history_generative",
    "mmlu_high_school_european_history_generative",
    "mmlu_high_school_world_history_generative",
    "mmlu_high_school_macroeconomics_generative",
    "mmlu_high_school_microeconomics_generative",
    "mmlu_high_school_psychology_generative",
    "mmlu_high_school_mathematics_generative",
    "mmlu_high_school_physics_generative",
    "mmlu_business_ethics_generative",
    "mmlu_moral_disputes_generative",
    "mmlu_moral_scenarios_generative",
    "mmlu_college_mathematics_generative",
    "mmlu_elementary_mathematics_generative",
    "mmlu_formal_logic_generative",
    "mmlu_abstract_algebra_generative",
    "mmlu_high_school_biology_generative",
    "mmlu_high_school_chemistry_generative",
    "mmlu_electrical_engineering_generative",
    "mmlu_college_chemistry_generative",
    "mmlu_college_physics_generative",
    "mmlu_college_biology_generative",
    "mmlu_college_medicine_generative",
    "mmlu_virology_generative",
    "mmlu_high_school_computer_science_generative",
    "mmlu_machine_learning_generative",
    "mmlu_computer_security_generative",
    "mmlu_college_computer_science_generative",
    "mmlu_miscellaneous_generative",
    "mmlu_conceptual_physics_generative",
    "mmlu_prehistory_generative",
    "mmlu_international_law_generative",
    "mmlu_human_aging_generative",
    "mmlu_security_studies_generative",
    "mmlu_astronomy_generative",
    "mmlu_logical_fallacies_generative",
    # MMLU PRO
    "mmlu_pro-category_other",
    "mmlu_pro-category_physics",
    "mmlu_pro-category_chemistry",
    "mmlu_pro-category_biology",
    "mmlu_pro-category_psychology",
    "mmlu_pro-category_health",
    "mmlu_pro-category_business",
    "mmlu_pro-category_law",
    "mmlu_pro-category_history",
    "mmlu_pro-category_philosophy",
    "mmlu_pro-category_economics",
    "mmlu_pro-category_math",
    "mmlu_pro-category_engineering",
    "mmlu_pro-category_computer-science",
    # IFEVAL
    "ifeval",
    # BBH
    "bbh_cot_fewshot_tracking_shuffled_objects_three_objects",
    "bbh_cot_fewshot_tracking_shuffled_objects_five_objects",
    "bbh_cot_fewshot_tracking_shuffled_objects_seven_objects",
    "bbh_cot_fewshot_dyck_languages",
    "bbh_cot_fewshot_word_sorting",
    "bbh_cot_fewshot_object_counting",
    "bbh_cot_fewshot_reasoning_about_colored_objects",
    "bbh_cot_fewshot_multistep_arithmetic_two",
    "bbh_cot_fewshot_penguins_in_a_table",
    "bbh_cot_fewshot_movie_recommendation",
    "bbh_cot_fewshot_navigate",
    "bbh_cot_fewshot_logical_deduction_three_objects",
    "bbh_cot_fewshot_logical_deduction_five_objects",
    "bbh_cot_fewshot_logical_deduction_seven_objects",
    "bbh_cot_fewshot_causal_judgement",
    "bbh_cot_fewshot_date_understanding",
    "bbh_cot_fewshot_temporal_sequences",
    "bbh_cot_fewshot_formal_fallacies",
    "bbh_cot_fewshot_boolean_expressions",
    "bbh_cot_fewshot_sports_understanding",
    "bbh_cot_fewshot_disambiguation_qa",
    "bbh_cot_fewshot_hyperbaton",
    "bbh_cot_fewshot_salient_translation_error_detection",
    "bbh_cot_fewshot_snarks",
    "bbh_cot_fewshot_web_of_lies",
    "bbh_cot_fewshot_ruin_names",
    # bAbI
    "babi-task_01-single_supporting_fact",
    "babi-task_02-two_supporting_facts",
    "babi-task_03-three_supporting_facts",
    "babi-task_04-two_argument_relations",
    "babi-task_05-three_argument_relations",
    "babi-task_06-yes_no_questions",
    "babi-task_07-counting",
    "babi-task_08-lists_sets",
    "babi-task_09-simple_negation",
    "babi-task_10-indefinite_knowledge",
    "babi-task_11-basic_coreference",
    "babi-task_12-conjunction",
    "babi-task_13-compound_coreference",
    "babi-task_14-time_reasoning",
    "babi-task_15-basic_deduction",
    "babi-task_16-basic_induction",
    "babi-task_17-positional_reasoning",
    "babi-task_18-size_reasoning",
    "babi-task_19-path_finding",
    "babi-task_20-agents_motivations",
]

babisteps_taxonomy_tasks = [
    "babisteps-task_01-simpletracking",
    "babisteps-task_02-immediateorder",
    "babisteps-task_03-complextracking",
    "babisteps-task_04-listing",
    "babisteps-task_05-sizeorder",
    "babisteps-task_06-spatialorder",
    "babisteps-task_07-temporalorder",
]

babisteps_chat_taxonomy_tasks = [
    "babisteps-chat-cot-task_01-simpletracking",
    "babisteps-chat-cot-task_02-immediateorder",
    "babisteps-chat-cot-task_03-complextracking",
    "babisteps-chat-cot-task_04-listing",
    "babisteps-chat-cot-task_05-sizeorder",
    "babisteps-chat-cot-task_06-spatialorder",
    "babisteps-chat-cot-task_07-temporalorder",
]
all_leaderboard_tasks = [
    # # MATH TODO : Abuse of splits probably...
    "leaderboard_math_algebra_hard",
    "leaderboard_math_counting_and_prob_hard",
    "leaderboard_math_geometry_hard",
    "leaderboard_math_intermediate_algebra_hard",
    "leaderboard_math_num_theory_hard",
    "leaderboard_math_prealgebra_hard",
    "leaderboard_math_precalculus_hard",
    # NOTE: All the others `leaderboard_<task>` are the `multiple_choice` tasks and then require
    # the loglikelihoods/tokenizers to compute the scores.
    # For now, we will not trigger them.
    # GPQA TODO : Checkear abuso aca tambien
    # "leaderboard_gpqa_main",
    # "leaderboard_gpqa_extended",
    # "leaderboard_gpqa_diamond", # TODO : Check why this particular task cannot be processed
    # # MUSR TODO : Split into proper datasets, do not abuse split
    # "leaderboard_musr_team_allocation",
    # "leaderboard_musr_murder_mysteries",
    # "leaderboard_musr_object_placements",
    # # MMLU-Pro (Covered by taxonomy and made by task as it should)
    # "leaderboard_mmlu_pro",
    # # BBH (covered by taxonomy)
    # "leaderboard_bbh_formal_fallacies",
    # "leaderboard_bbh_navigate",
    # "leaderboard_bbh_sports_understanding",
    # "leaderboard_bbh_object_counting",
    # "leaderboard_bbh_temporal_sequences",
    # "leaderboard_bbh_penguins_in_a_table",
    # "leaderboard_bbh_tracking_shuffled_objects_five_objects",
    # "leaderboard_bbh_geometric_shapes",
    # "leaderboard_bbh_hyperbaton",
    # "leaderboard_bbh_boolean_expressions",
    # "leaderboard_bbh_logical_deduction_five_objects",
    # "leaderboard_bbh_ruin_names",
    # "leaderboard_bbh_tracking_shuffled_objects_seven_objects",
    # "leaderboard_bbh_reasoning_about_colored_objects",
    # "leaderboard_bbh_tracking_shuffled_objects_three_objects",
    # "leaderboard_bbh_salient_translation_error_detection",
    # "leaderboard_bbh_web_of_lies",
    # "leaderboard_bbh_logical_deduction_seven_objects",
    # "leaderboard_bbh_logical_deduction_three_objects",
    # "leaderboard_bbh_snarks",
    # "leaderboard_bbh_movie_recommendation",
    # "leaderboard_bbh_date_understanding",
    # "leaderboard_bbh_causal_judgement",
    # "leaderboard_bbh_disambiguation_qa",
]


def run_command(command):
    try:
        subprocess.run(command, check=True)
        return True
    except subprocess.CalledProcessError as e:
        print(f"Error executing command: {e}")
        return False


def schedule_lookup_task(interval="1m", execution_timeout=350, task_timeout=175):
    command = BASE_COMMAND + [
        "--",
        "temporal",
        "schedule",
        "create",
        "--schedule-id",
        "lookup-done-tasks",
        "--workflow-id",
        "lookup-done-tasks",
        "--type",
        "LookupTasks",
        "--task-queue",
        "evaluator",
        "--interval",
        f"{interval}",
        "--overlap-policy",
        "Skip",
        "--execution-timeout",
        f"{execution_timeout}s",
        "--task-timeout",
        f"{task_timeout}s",
        "--namespace",
        f"{TEMPORAL_NAMESPACE}",
    ]
    return run_command(command)


def schedule_taxonomy_summary_task(
    interval="1h", execution_timeout=1200, task_timeout=1200
):
    command = BASE_COMMAND + [
        "--",
        "temporal",
        "schedule",
        "create",
        "--schedule-id",
        "taxonomy-summary-lookup",
        "--workflow-id",
        "taxonomy-summary-lookup",
        "--type",
        "TaxonomySummaryLookup",
        "--task-queue",
        "summarize",
        "--interval",
        f"{interval}",
        "--overlap-policy",
        "Skip",
        "--execution-timeout",
        f"{execution_timeout}s",
        "--task-timeout",
        f"{task_timeout}s",
        "--namespace",
        f"{TEMPORAL_NAMESPACE}",
    ]
    return run_command(command)


def schedule_requester_task(
    app_address, chain_id, interval="1m", execution_timeout=350, task_timeout=175
):
    command = BASE_COMMAND + [
        "--",
        "temporal",
        "schedule",
        "create",
        "--schedule-id",
        f"requester-{chain_id}-{app_address}",
        "--workflow-id",
        f"requester-{chain_id}-{app_address}",
        "--type",
        "Requester",
        "--task-queue",
        "requester",
        "--interval",
        f"{interval}",
        "--overlap-policy",
        "Skip",
        "--execution-timeout",
        f"{execution_timeout}s",
        "--task-timeout",
        f"{task_timeout}s",
        "--namespace",
        f"{TEMPORAL_NAMESPACE}",
        "--input",
        f'{{"app":"{app_address}","service":"{chain_id}"}}',
    ]
    return run_command(command)


def schedule_tokenizer_task(
    chain_id, interval="2m", execution_timeout=120, task_timeout=120
):
    command = BASE_COMMAND + [
        "--",
        "temporal",
        "schedule",
        "create",
        "--schedule-id",
        f"signatures-tokenizer-{chain_id}",
        "--workflow-id",
        f"signatures-tokenizer-{chain_id}",
        "--type",
        "Manager",
        "--task-queue",
        "manager",
        "--interval",
        f"{interval}",
        "--overlap-policy",
        "Skip",
        "--execution-timeout",
        f"{execution_timeout}s",
        "--task-timeout",
        f"{task_timeout}s",
        "--namespace",
        f"{TEMPORAL_NAMESPACE}",
        "--input",
        f'{{"service":"{chain_id}","tests":[{{"framework" : "signatures", "tasks": ["tokenizer"]}}]}}',
    ]
    return run_command(command)


def schedule_config_task(
    chain_id, interval="2m", execution_timeout=120, task_timeout=120
):
    command = BASE_COMMAND + [
        "--",
        "temporal",
        "schedule",
        "create",
        "--schedule-id",
        f"signatures-config-{chain_id}",
        "--workflow-id",
        f"signatures-config-{chain_id}",
        "--type",
        "Manager",
        "--task-queue",
        "manager",
        "--interval",
        f"{interval}",
        "--overlap-policy",
        "Skip",
        "--execution-timeout",
        f"{execution_timeout}s",
        "--task-timeout",
        f"{task_timeout}s",
        "--namespace",
        f"{TEMPORAL_NAMESPACE}",
        "--input",
        f'{{"service":"{chain_id}","tests":[{{"framework" : "signatures", "tasks": ["config"]}}]}}',
    ]
    return run_command(command)


def schedule_benchmark_task(
    benchmark, chain_id, interval="2m", execution_timeout=120, task_timeout=120
):
    command = BASE_COMMAND + [
        "--",
        "temporal",
        "schedule",
        "create",
        "--schedule-id",
        f"{LMEH_TYPE}-{benchmark}-{chain_id}",
        "--workflow-id",
        f"{LMEH_TYPE}-{benchmark}-{chain_id}",
        "--type",
        "Manager",
        "--task-queue",
        "manager",
        "--interval",
        f"{interval}",
        "--overlap-policy",
        "Skip",
        "--execution-timeout",
        f"{execution_timeout}s",
        "--task-timeout",
        f"{task_timeout}s",
        "--namespace",
        f"{TEMPORAL_NAMESPACE}",
        "--input",
        f'{{"service":"{chain_id}","tests":[{{"framework" : "{LMEH_TYPE}", "tasks": ["{benchmark}"]}}]}}',
    ]
    return run_command(command)


def execute_register_task(task, execution_timeout=7200, task_timeout=3600):
    """
    Execute the Docker command to start a Temporal workflow.

    Args:
        key (str): The task key to be passed as input to the workflow.
    """
    command = BASE_COMMAND + [
        "--",
        "temporal",
        "workflow",
        "start",
        "--task-queue",
        "sampler",
        "--type",
        "Register",
        "--input",
        f'{{"framework": "{LMEH_TYPE}", "tasks": "{task}"}}',
        "--execution-timeout",
        f"{execution_timeout}s",
        "--task-timeout",
        f"{task_timeout}s",
        "--namespace",
        f"{TEMPORAL_NAMESPACE}",
    ]
    return run_command(command)


def parse_dict_from_string(arg_string):
    """Parses a string representation of a dictionary into a Python dictionary."""
    try:
        return json.loads(arg_string)
    except json.JSONDecodeError:
        raise argparse.ArgumentTypeError(
            f"Invalid dictionary format: '{arg_string}'. Please use valid JSON syntax."
        )


def main():
    global BASE_COMMAND, TEMPORAL_NAMESPACE, APPS_PER_SERVICE, LMEH_TYPE

    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--only-registers", action="store_true", help="Only trigger register tasks"
    )
    parser.add_argument(
        "--generative", action="store_true", help="Use generative LMEH tasks"
    )
    parser.add_argument(
        "--task", help="optionally pass a task identifier, e.g. --task ifeval-fix"
    )
    parser.add_argument(
        "--k8s-namespace", help="Namespace of the k8s deployment, defaults to default"
    )
    parser.add_argument(
        "--temporal-namespace",
        help=f"Namespace of temporal, defaults to {TEMPORAL_NAMESPACE}",
    )
    parser.add_argument(
        "--pokt-service-apps",
        type=parse_dict_from_string,
        help='A dictionary in JSON format (e.g., \'{"lm": ["pokt1wkra80yv9zv69y2rgkmc69jfqph6053dwn47vx"]}\')',
    )
    parser.add_argument(
        "--framework-postfix",
        help='Optional: Framework postfix to use, the final framework name will be "lmeh-THISVALUE"',
    )

    args = parser.parse_args()

    if args.pokt_service_apps:
        print("Received services and apps:", args.pokt_service_apps)
        if isinstance(args.pokt_service_apps, dict):
            APPS_PER_SERVICE = args.pokt_service_apps

    if args.k8s_namespace:
        print(f"Using k8s Namespace: {args.k8s_namespace}")
        BASE_COMMAND += ["-n", f"{args.k8s_namespace}"]
    if args.temporal_namespace:
        print(f"Using Temporal Namespace: {args.temporal_namespace}")
        TEMPORAL_NAMESPACE = args.temporal_namespace

    total_registers = 0
    total_tokenizers = 0
    total_configs = 0
    total_requesters = 0
    total_benchmarks = 0

    all_tasks = (
        all_taxonomy_tasks + all_leaderboard_tasks + babisteps_chat_taxonomy_tasks
    )
    if args.task:
        print(f"Triggering only: {args.task}")
        all_tasks = [args.task]

    if args.framework_postfix:
        LMEH_TYPE += "-" + args.framework_postfix
    elif args.generative:
        LMEH_TYPE += "-generative"

    if args.only_registers:
        print("Setting-up registers only:")
        for task in all_tasks:
            # Register dataset
            print(f"\t{task}")
            ok = execute_register_task(task, execution_timeout=7200, task_timeout=3600)
            time.sleep(0.25)
            total_registers += ok

    else:
        # Start the base task lookup
        schedule_lookup_task(interval="1m", execution_timeout=350, task_timeout=175)
        print("Lookup scheduled.")
        time.sleep(0.25)

        schedule_taxonomy_summary_task(
            interval="1h", execution_timeout=1200, task_timeout=1200
        )
        print("Taxonomy summary scheduled.")
        time.sleep(0.25)

        # Create per-service tasks
        if not args.generative:
            for chain_id in APPS_PER_SERVICE.keys():
                print(f"Triggering signatures for {chain_id}:")
                # Schedule the tokenizer in this service ID
                ok = schedule_tokenizer_task(
                    chain_id, interval="2m", execution_timeout=120, task_timeout=120
                )
                print("\tTokenizer triggered.")
                time.sleep(0.25)
                total_tokenizers += ok
                # Schedule the config task in this Service ID
                ok = schedule_config_task(
                    chain_id, interval="2m", execution_timeout=120, task_timeout=120
                )
                print("\tConfiguration triggered.")
                time.sleep(0.25)
                total_configs += ok
                # Schedule a requester for each app in this service ID
                print("\tTriggering requesters for apps:")
                for app in APPS_PER_SERVICE[chain_id]:
                    # Schedule the requester using this app
                    ok = schedule_requester_task(
                        app,
                        chain_id,
                        interval="1m",
                        execution_timeout=350,
                        task_timeout=175,
                    )
                    total_requesters += ok
                    print(f"\t\t{app}")
                    time.sleep(0.25)
            print("Signatures scheduled.")

        # Create per-service tasks
        for chain_id in APPS_PER_SERVICE.keys():
            print(f"Triggering requesters for {chain_id} apps':")
            for app in APPS_PER_SERVICE[chain_id]:
                print(f"\t{app}")
                # Schedule the requester using this app
                ok = schedule_requester_task(
                    app,
                    chain_id,
                    interval="1m",
                    execution_timeout=350,
                    task_timeout=175,
                )
                total_requesters += ok
                print(f"\t\t{app}")
                time.sleep(0.25)
        print("Requesters scheduled.")

        # Create all tasks for all chains
        for task in all_tasks:
            print(f"Setting-up task: {task}")
            # Register dataset
            ok = execute_register_task(task, execution_timeout=7200, task_timeout=3600)
            print("\tRegistering triggered.")
            time.sleep(0.25)
            total_registers += ok

            # Finally schedule the benchmark
            for chain_id in APPS_PER_SERVICE.keys():
                ok = schedule_benchmark_task(
                    task,
                    chain_id,
                    interval="2m",
                    execution_timeout=120,
                    task_timeout=120,
                )
                print("\tTask triggered.")
                time.sleep(0.25)
                total_benchmarks += ok

    total_tasks = {
        "Registers": total_registers,
        "Tokenizers": total_tokenizers,
        "Configs": total_configs,
        "Requesters": total_requesters,
        "Benchmarks": total_benchmarks,
    }

    # Calculate total triggered tasks
    total_triggered_tasks = sum(total_tasks.values())

    # Print total triggered tasks
    print("Total Triggered Tasks:")
    print("-----------------------")
    for task, total in total_tasks.items():
        print(f"{task:15}: {total:5}")
    print("-----------------------")
    print(f"Total:           {total_triggered_tasks:5}")


# Example usage:
if __name__ == "__main__":
    main()
