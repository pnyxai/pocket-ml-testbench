import subprocess
import time
import argparse
import json
import os
import sys
import math
import re

sys.path.append("../")
from packages.python.taxonomies.utils import load_taxonomy, get_taxonomy_datasets


TEMPORAL_NAMESPACE = "pocket-ml-testbench"
APPS_PER_SERVICE = {
    "lm": ["pokt1wkra80yv9zv69y2rgkmc69jfqph6053dwn47vx"],
}

DEPLOYMENT_NAME = "deploy/temporal-admintools"

BASE_COMMAND = ["kubectl", "exec", "-it"]

LMEH_TYPE = "lmeh"


def run_command(command):
    try:
        subprocess.run(command, check=True)
        return True
    except subprocess.CalledProcessError as e:
        print(f"Error executing command: {e}")
        return False


def schedule_lookup_task(interval="1m", execution_timeout=600, task_timeout=540):
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
        "--catchup-window",
        "1s",
        "--execution-timeout",
        f"{execution_timeout}s",
        "--task-timeout",
        f"{task_timeout}s",
        "--namespace",
        f"{TEMPORAL_NAMESPACE}",
    ]
    return run_command(command)


def schedule_snapshot_task(interval="24h", execution_timeout=1200, task_timeout=1200):
    command = BASE_COMMAND + [
        "--",
        "temporal",
        "schedule",
        "create",
        "--schedule-id",
        "supplier-snapshot-lookup",
        "--workflow-id",
        "supplier-snapshot-lookup",
        "--type",
        "SuppliersSnapshotLookup",
        "--task-queue",
        "summarize",
        "--interval",
        f"{interval}",
        "--overlap-policy",
        "Skip",
        "--catchup-window",
        "1s",
        "--execution-timeout",
        f"{execution_timeout}s",
        "--task-timeout",
        f"{task_timeout}s",
        "--namespace",
        f"{TEMPORAL_NAMESPACE}",
    ]
    return run_command(command)


def schedule_summary_task(interval="1h", execution_timeout=1200, task_timeout=1200):
    command = BASE_COMMAND + [
        "--",
        "temporal",
        "schedule",
        "create",
        "--schedule-id",
        "summary-lookup",
        "--workflow-id",
        "summary-lookup",
        "--type",
        "SummaryLookup",
        "--task-queue",
        "summarize",
        "--interval",
        f"{interval}",
        "--overlap-policy",
        "Skip",
        "--catchup-window",
        "1s",
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
        "--catchup-window",
        "1s",
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
        "--catchup-window",
        "1s",
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
        "--catchup-window",
        "1s",
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


def schedule_identity_task(
    chain_id, interval="2m", execution_timeout=120, task_timeout=120
):
    command = BASE_COMMAND + [
        "--",
        "temporal",
        "schedule",
        "create",
        "--schedule-id",
        f"signatures-identity-{chain_id}",
        "--workflow-id",
        f"signatures-identity-{chain_id}",
        "--type",
        "Manager",
        "--task-queue",
        "manager",
        "--interval",
        f"{interval}",
        "--overlap-policy",
        "Skip",
        "--catchup-window",
        "1s",
        "--execution-timeout",
        f"{execution_timeout}s",
        "--task-timeout",
        f"{task_timeout}s",
        "--namespace",
        f"{TEMPORAL_NAMESPACE}",
        "--input",
        f'{{"service":"{chain_id}","tests":[{{"framework" : "signatures", "tasks": ["identity"]}}]}}',
    ]
    return run_command(command)


def schedule_benchmark_task(
    benchmark, chain_id, interval="2m", execution_timeout=120, task_timeout=120, phase=0
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
        "--catchup-window",
        "1s",
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


def validate_interval(interval_str):
    """Validates that interval string has correct format (e.g., '5m', '1h', '24h')."""
    import re

    if not re.match(r"^\d+[smhd]$", interval_str):
        raise argparse.ArgumentTypeError(
            f"Invalid interval format: '{interval_str}'. Must be a number followed by 's', 'm', 'h', or 'd' (e.g., '10s', '5m', '1h', '24h')."
        )
    return interval_str


def parse_interval_to_seconds(interval_str):
    """Convert interval string (e.g., '5m', '1h') to total seconds."""
    match = re.match(r"^(\d+)([smhd])$", interval_str)
    if not match:
        raise ValueError(f"Invalid interval format: '{interval_str}'")

    value, unit = int(match.group(1)), match.group(2)

    if unit == "s":
        return value
    elif unit == "m":
        return value * 60
    elif unit == "h":
        return value * 3600
    elif unit == "d":
        return value * 86400
    else:
        raise ValueError(f"Unknown time unit: '{unit}'")


def main():
    global \
        BASE_COMMAND, \
        TEMPORAL_NAMESPACE, \
        APPS_PER_SERVICE, \
        LMEH_TYPE, \
        DEPLOYMENT_NAME

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
        "--taxonomy", help="optionally pass a taxonomy name, e.g. --taxonomy general"
    )
    parser.add_argument(
        "--k8s-namespace",
        help="Namespace of the k8s deployment, defaults to not specified (default)",
    )
    parser.add_argument(
        "--k8s-deployment",
        help=f"Name of the k8s deployment, defaults to {DEPLOYMENT_NAME}",
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
    parser.add_argument(
        "--identity", action="store_true", help="Trigger identity signature tasks"
    )
    parser.add_argument(
        "--only-requesters", action="store_true", help="Only trigger requester tasks"
    )
    parser.add_argument(
        "--benchmark-interval",
        type=validate_interval,
        default="5m",
        help="Interval for benchmark tasks (default: 5m)",
    )
    parser.add_argument(
        "--tokenizer-interval",
        type=validate_interval,
        default="2m",
        help="Interval for tokenizer tasks (default: 2m)",
    )
    parser.add_argument(
        "--config-interval",
        type=validate_interval,
        default="2m",
        help="Interval for config tasks (default: 2m)",
    )
    parser.add_argument(
        "--identity-interval",
        type=validate_interval,
        default="2m",
        help="Interval for identity tasks (default: 2m)",
    )
    parser.add_argument(
        "--requester-interval",
        type=validate_interval,
        default="1m",
        help="Interval for requester tasks (default: 1m)",
    )
    parser.add_argument(
        "--lookup-interval",
        type=validate_interval,
        default="10m",
        help="Interval for lookup tasks (default: 10m)",
    )
    parser.add_argument(
        "--summary-interval",
        type=validate_interval,
        default="1h",
        help="Interval for summary tasks (default: 1h)",
    )
    parser.add_argument(
        "--snapshot-interval",
        type=validate_interval,
        default="24h",
        help="Interval for snapshot tasks (default: 24h)",
    )
    parser.add_argument(
        "--phase-offset",
        type=int,
        default=0,
        help="Custom phase offset in seconds to add to calculated phase (default: 0)",
    )

    args = parser.parse_args()

    # Extract interval values
    benchmark_interval = args.benchmark_interval
    tokenizer_interval = args.tokenizer_interval
    config_interval = args.config_interval
    identity_interval = args.identity_interval
    requester_interval = args.requester_interval
    lookup_interval = args.lookup_interval
    summary_interval = args.summary_interval
    snapshot_interval = args.snapshot_interval
    phase_offset = args.phase_offset

    # Validate taxonomy if provided
    if args.taxonomy:
        # Check if path exists
        if not os.path.exists(args.taxonomy):
            print(f"Error: Taxonomy file '{args.taxonomy}' not found.")
            exit(1)
        # Load taxonomy
        taxonomy_graph, _, _, _ = load_taxonomy(
            args.taxonomy, return_all=True, verbose=True, print_prefix="\t"
        )

    # Check for conflicting arguments
    if args.task and args.taxonomy:
        print("Error: --task and --taxonomy arguments cannot be used together.")
        print(
            "Please specify either a single task with --task or a taxonomy with --taxonomy."
        )
        return

    # Require at least one of --task, --taxonomy, --identity, or --only-requesters
    if (
        not args.task
        and not args.taxonomy
        and not args.identity
        and not args.only_requesters
    ):
        print(
            "Error: Either --task, --taxonomy, --identity, or --only-requesters must be specified."
        )
        print(
            "Please specify either a single task with --task, a taxonomy with --taxonomy, identity signature with --identity, or requesters with --only-requesters."
        )
        return

    if args.pokt_service_apps:
        print("Received services and apps:", args.pokt_service_apps)
        if isinstance(args.pokt_service_apps, dict):
            APPS_PER_SERVICE = args.pokt_service_apps

    if args.k8s_deployment:
        print(f"Using k8s Namespace: {args.k8s_namespace}")
        DEPLOYMENT_NAME = args.k8s_deployment
    BASE_COMMAND += [DEPLOYMENT_NAME]

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

    # Determine which tasks to process
    if args.task:
        print(f"Triggering only task: {args.task}")
        tasks_to_process = [args.task]
    elif args.taxonomy:
        print(f"Triggering taxonomy: {args.taxonomy}")
        tasks_to_process = get_taxonomy_datasets(taxonomy_graph)
    else:
        # For --only-requesters or --identity, tasks_to_process is not needed
        tasks_to_process = []

    # This flag is necessary to set correct dependencies and is independent of
    # any other postfix used
    if args.generative:
        LMEH_TYPE += "-generative"
    if args.framework_postfix:
        LMEH_TYPE += "-" + args.framework_postfix

    trigger_requesters = False
    if args.only_registers:
        print("Setting-up registers only:")
        for task in tasks_to_process:
            # Register dataset
            print(f"\t{task}")
            ok = execute_register_task(task, execution_timeout=7200, task_timeout=3600)
            time.sleep(0.25)
            total_registers += ok

    elif args.only_requesters:
        print("Setting-up requesters only:")
        trigger_requesters = True

    elif args.identity:
        trigger_requesters = True
        for chain_id in APPS_PER_SERVICE.keys():
            ok = schedule_identity_task(
                chain_id,
                interval=identity_interval,
                execution_timeout=120,
                task_timeout=120,
            )
            total_registers += ok

    else:
        trigger_requesters = True
        # Create per-service tasks
        if not args.generative:
            for chain_id in APPS_PER_SERVICE.keys():
                print(f"Triggering signatures for {chain_id}:")
                # Schedule the tokenizer in this service ID
                ok = schedule_tokenizer_task(
                    chain_id,
                    interval=tokenizer_interval,
                    execution_timeout=120,
                    task_timeout=120,
                )
                print("\tTokenizer triggered.")
                time.sleep(0.25)
                total_tokenizers += ok
                # Schedule the config task in this Service ID
                ok = schedule_config_task(
                    chain_id,
                    interval=config_interval,
                    execution_timeout=120,
                    task_timeout=120,
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
                        interval=requester_interval,
                        execution_timeout=350,
                        task_timeout=175,
                    )
                    total_requesters += ok
                    print(f"\t\t{app}")
                    time.sleep(0.25)
            print("Signatures scheduled.")

        # Calculate phase staggering for benchmark tasks
        num_tasks = len(tasks_to_process)
        phase_per_task = 0
        if num_tasks > 0:
            try:
                interval_seconds = parse_interval_to_seconds(benchmark_interval)
                phase_per_task = max(1, math.ceil(interval_seconds / num_tasks))
            except ValueError as e:
                print(f"Warning: Failed to calculate phase: {e}")
                phase_per_task = 0
        elif num_tasks == 0:
            print("Warning: No tasks to process, phase staggering set to 0")
            phase_per_task = 0

        # Create all tasks for all chains
        for task_index, task in enumerate(tasks_to_process):
            if "undefined_task" in task:
                print(f"Skipping undefined task: {task}")
                continue
            print(f"Setting-up task: {task}")
            # Register dataset
            ok = execute_register_task(task, execution_timeout=7200, task_timeout=3600)
            print("\tRegistering triggered.")
            time.sleep(0.25)
            total_registers += ok

            # Finally schedule the benchmark
            for chain_id in APPS_PER_SERVICE.keys():
                current_phase = task_index * phase_per_task + phase_offset
                ok = schedule_benchmark_task(
                    task,
                    chain_id,
                    interval=benchmark_interval,  # if "trigger_minimum" is set to >0 in the manager, this will define the sampling rate
                    execution_timeout=240,
                    task_timeout=240,
                    phase=current_phase,
                )
                print("\tTask triggered.")
                time.sleep(0.25)
                total_benchmarks += ok

    if trigger_requesters:
        # Start the base task lookup
        schedule_lookup_task(
            interval=lookup_interval, execution_timeout=550, task_timeout=500
        )
        print("Lookup scheduled.")
        time.sleep(0.25)

        schedule_summary_task(
            interval=summary_interval, execution_timeout=1200, task_timeout=1200
        )
        print("Summary scheduled.")
        time.sleep(0.25)

        schedule_snapshot_task(
            interval=snapshot_interval, execution_timeout=1200, task_timeout=1200
        )
        print("Snapshot scheduled.")
        time.sleep(0.25)

        # Create per-service tasks
        for chain_id in APPS_PER_SERVICE.keys():
            print(f"Triggering requesters for {chain_id} apps':")
            for app in APPS_PER_SERVICE[chain_id]:
                print(f"\t{app}")
                # Schedule the requester using this app
                ok = schedule_requester_task(
                    app,
                    chain_id,
                    interval=requester_interval,
                    execution_timeout=350,
                    task_timeout=175,
                )
                total_requesters += ok
                print(f"\t\t{app}")
                time.sleep(0.25)
        print("Requesters scheduled.")

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
