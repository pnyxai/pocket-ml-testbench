import subprocess
import argparse
import sys
import time
import os
import re


# Default configuration
DEPLOYMENT_NAME = "deploy/temporal-admintools"
POSTGRESQL_POD_NAME = "postgresql"
TEMPORAL_NAMESPACE = "pocket-ml-testbench"
POSTGRES_NAMESPACE = None  # If not set, uses TEMPORAL_NAMESPACE
POSTGRES_HOST = "postgresql-service"
POSTGRES_PORT = "5432"
POSTGRES_USER = "testbench"
POSTGRES_DB = "pocket-ml-testbench"
POSTGRES_PASSWORD = None  # Must be set from env or args

PSQL_COMMAND = ["kubectl", "exec", "-it", POSTGRESQL_POD_NAME, "--"]
TEMPORAL_BASE_COMMAND = ["kubectl", "exec", "-it", DEPLOYMENT_NAME, "--"]

LMEH_TYPE = "lmeh"


def run_command(command):
    """Execute a command."""
    try:
        subprocess.run(command, check=True)
        return True
    except subprocess.CalledProcessError as e:
        print(f"Error executing command: {e}")
        return False


def run_command_with_output(command):
    """Execute a command and capture its output."""

    try:
        result = subprocess.run(command, check=True, capture_output=True, text=True, stdin=subprocess.PIPE, timeout=30)
        return result.stdout
    except subprocess.TimeoutExpired as e:
        print(f"Timeout executing command: {e}")
        return None
    except subprocess.CalledProcessError as e:
        print(f"Error executing command: {e}")
        return None


def get_dataset_table_name(task_name):
    """
    Query task_registry to find the dataset table name for a given task.
    Returns the table name or None if task not found.
    """
    # Build psql command to query task_registry
    psql_query = (
        f"SELECT dataset_table_name FROM task_registry WHERE task_name = '{task_name}';"
    )

    # Build connection URI with password
    postgres_uri = f"postgresql://{POSTGRES_USER}:{POSTGRES_PASSWORD}@{POSTGRES_HOST}:{POSTGRES_PORT}/{POSTGRES_DB}"

    command = PSQL_COMMAND + [
        "psql",
        postgres_uri,
        "-t",  # Tuples only (no headers)
        "-c",
        psql_query,
    ]
    result = run_command_with_output(command)
    if result is not None:
        table_name = result.strip()
        return table_name
    else:
        return None


def get_record_count(table_name):
    """
    Query the record count in a dataset table.
    Returns the count or None if unable to retrieve.
    """
    psql_query = f'SELECT COUNT(*) FROM "{table_name}";'

    # Build connection URI with password
    postgres_uri = f"postgresql://{POSTGRES_USER}:{POSTGRES_PASSWORD}@{POSTGRES_HOST}:{POSTGRES_PORT}/{POSTGRES_DB}"

    command = PSQL_COMMAND + [
        "psql",
        postgres_uri,
        "-t",  # Tuples only
        "-c",
        psql_query,
    ]

    result = run_command_with_output(command)
    if result is not None:

        count = result.strip()
        return int(count) if count else 0
    else:
        print(f"Error querying record count.")
        return None


def delete_dataset_table(table_name):
    """
    Delete (drop) the dataset table from PostgreSQL.
    """
    psql_query = f'DROP TABLE IF EXISTS "{table_name}" CASCADE;'

    # Build connection URI with password
    postgres_uri = f"postgresql://{POSTGRES_USER}:{POSTGRES_PASSWORD}@{POSTGRES_HOST}:{POSTGRES_PORT}/{POSTGRES_DB}"

    command = PSQL_COMMAND + [
        "psql",
        postgres_uri,
        "-c",
        psql_query,
    ]

    return run_command(command)


def unregister_task(task_name):
    """
    Unregister a task from task_registry.
    """
    psql_query = f"DELETE FROM task_registry WHERE task_name = '{task_name}';"

    # Build connection URI with password
    postgres_uri = f"postgresql://{POSTGRES_USER}:{POSTGRES_PASSWORD}@{POSTGRES_HOST}:{POSTGRES_PORT}/{POSTGRES_DB}"

    command = PSQL_COMMAND + [
        "psql",
        postgres_uri,
        "-c",
        psql_query,
    ]

    return run_command(command)


def execute_register_task(task, execution_timeout=600, task_timeout=600):
    """
    Execute the Docker command to start a Temporal workflow for registering a task.

    Args:
        task (str): The task key to be passed as input to the workflow.
        execution_timeout (int): Execution timeout in seconds (default: 600s = 10 min).
        task_timeout (int): Task timeout in seconds (default: 600s = 10 min).
    """
    command = TEMPORAL_BASE_COMMAND + [
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
        "--run-timeout",
        f"{execution_timeout}s",
        "--task-timeout",
        f"{task_timeout}s",
        "--namespace",
        f"{TEMPORAL_NAMESPACE}",
    ]
    return run_command(command)


def confirm_deletion(task_names, table_names, record_counts):
    """
    Display tasks to be refreshed and ask for confirmation.

    Args:
        task_names (list): List of task names to be refreshed.
        table_names (list): List of dataset table names to be deleted.
        record_counts (list): List of record counts for each table.

    Returns:
        True if user confirms, False otherwise.
    """
    if not task_names:
        print("No tasks to refresh.")
        return False

    print("\nTasks to be refreshed:")
    print("-" * 80)
    for task, table, count in zip(task_names, table_names, record_counts):
        count_str = f"{count} records" if count is not None else "unknown records"
        print(f"\tTask:  {task}")
        print(f"\t\tTable: {table}")
        print(f"\t\tRecords: {count_str}")
        print()
    print("-" * 80)
    print(f"Total: {len(task_names)} task(s)")

    while True:
        response = input("\nProceed with refresh? (yes/no): ").strip().lower()
        if response in ("yes", "y"):
            return True
        elif response in ("no", "n"):
            return False
        else:
            print("Please enter 'yes' or 'no'.")


def main():
    global \
        DEPLOYMENT_NAME, \
        POSTGRESQL_POD_NAME, \
        TEMPORAL_NAMESPACE, \
        POSTGRES_NAMESPACE, \
        POSTGRES_HOST, \
        POSTGRES_PORT, \
        POSTGRES_USER, \
        POSTGRES_DB, \
        POSTGRES_PASSWORD, \
        PSQL_COMMAND, \
        TEMPORAL_BASE_COMMAND, \
        LMEH_TYPE

    parser = argparse.ArgumentParser(
        description="Refresh datasets of selected task(s) by deleting and re-registering them."
    )
    parser.add_argument(
        "--task",
        help="Task identifier(s) to refresh (comma-separated for multiple tasks, e.g. --task task1,task2,task3)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Preview deletions and register workflow without executing",
    )
    parser.add_argument(
        "--force",
        action="store_true",
        help="Skip confirmation prompt",
    )
    parser.add_argument(
        "--skip-register",
        action="store_true",
        help="Only delete datasets, skip re-registering",
    )
    parser.add_argument(
        "--k8s-namespace",
        help="Kubernetes namespace (optional)",
    )
    parser.add_argument(
        "--k8s-deployment",
        help=f"Kubernetes temporal deployment name (default: {DEPLOYMENT_NAME})",
    )
    parser.add_argument(
        "--postgres-deployment",
        help=f"Kubernetes PostgreSQL deployment/pod name (default: {POSTGRESQL_POD_NAME})",
    )
    parser.add_argument(
        "--postgres-namespace",
        help="Kubernetes PostgreSQL namespace (optional, defaults to --k8s-namespace if not specified)",
    )
    parser.add_argument(
        "--temporal-namespace",
        help=f"Temporal namespace (default: {TEMPORAL_NAMESPACE})",
    )
    parser.add_argument(
        "--postgres-host",
        help=f"PostgreSQL host (default: {POSTGRES_HOST})",
    )
    parser.add_argument(
        "--postgres-port",
        help=f"PostgreSQL port (default: {POSTGRES_PORT})",
    )
    parser.add_argument(
        "--postgres-user",
        help=f"PostgreSQL user (default: {POSTGRES_USER})",
    )
    parser.add_argument(
        "--postgres-password",
        help="PostgreSQL password (default: from POSTGRES_PASSWORD env var)",
    )
    parser.add_argument(
        "--postgres-db",
        help=f"PostgreSQL database (default: {POSTGRES_DB})",
    )
    parser.add_argument(
        "--register-timeout",
        type=int,
        default=600,
        help="Register task timeout in seconds (default: 600s = 10 min)",
    )
    parser.add_argument(
        "--generative",
        action="store_true",
        help="Use generative LMEH framework for re-registering",
    )
    parser.add_argument(
        "--framework-postfix",
        help="Optional framework postfix for re-registering (final name: lmeh-POSTFIX)",
    )

    args = parser.parse_args()

    # Parse comma-separated tasks
    if not args.task:
        print("Error: At least one --task must be specified.")
        print("Usage: python3 refresh_tasks.py --task task1,task2,task3")
        return

    # Split comma-separated tasks and strip whitespace
    tasks = [t.strip() for t in args.task.split(",")]
    tasks = [t for t in tasks if t]  # Remove empty strings

    if not tasks:
        print("Error: No valid tasks provided.")
        print("Usage: python3 refresh_tasks.py --task task1,task2,task3")
        return

    # Set PostgreSQL password from env variable or argument
    if args.postgres_password:
        POSTGRES_PASSWORD = args.postgres_password
    else:
        POSTGRES_PASSWORD = os.environ.get("POSTGRES_PASSWORD")
        if not POSTGRES_PASSWORD:
            print("Error: PostgreSQL password not provided.")
            print(
                "Please either set POSTGRES_PASSWORD environment variable or use --postgres-password argument."
            )
            return

    # Apply configuration overrides
    if args.postgres_host:
        POSTGRES_HOST = args.postgres_host
    if args.postgres_port:
        POSTGRES_PORT = args.postgres_port
    if args.postgres_user:
        POSTGRES_USER = args.postgres_user
    if args.postgres_db:
        POSTGRES_DB = args.postgres_db

    if args.k8s_deployment:
        DEPLOYMENT_NAME = args.k8s_deployment
    if args.postgres_deployment:
        POSTGRESQL_POD_NAME = args.postgres_deployment

    # Determine namespaces
    temporal_ns = args.k8s_namespace  # Temporal uses k8s-namespace by default
    postgres_ns = (
        args.postgres_namespace if args.postgres_namespace else args.k8s_namespace
    )  # Postgres uses postgres-namespace if provided, otherwise k8s-namespace

    # Build commands with proper k8s configuration
    TEMPORAL_BASE_COMMAND = ["kubectl", "exec", "-it", DEPLOYMENT_NAME, "--"]
    PSQL_COMMAND = ["kubectl", "exec", "-it", POSTGRESQL_POD_NAME, "--"]

    if temporal_ns:
        print(f"Using k8s Namespace for Temporal: {temporal_ns}")
        TEMPORAL_BASE_COMMAND = [
            "kubectl",
            "exec",
            "-it",
            DEPLOYMENT_NAME,
            "-n",
            f"{temporal_ns}",
            "--",
        ]

    if postgres_ns:
        print(f"Using k8s Namespace for PostgreSQL: {postgres_ns}")
        PSQL_COMMAND = [
            "kubectl",
            "exec",
            "-it",
            POSTGRESQL_POD_NAME,
            "-n",
            f"{postgres_ns}",
            "--",
        ]

    if args.temporal_namespace:
        print(f"Using Temporal Namespace: {args.temporal_namespace}")
        TEMPORAL_NAMESPACE = args.temporal_namespace

    if args.generative:
        LMEH_TYPE += "-generative"
    if args.framework_postfix:
        LMEH_TYPE += "-" + args.framework_postfix

    # Resolve task names and find their dataset tables
    print(f"\nResolving {len(tasks)} task(s)...")
    print("-" * 80)

    task_names = []
    table_names = []
    record_counts = []
    failed_tasks = []

    for task in tasks:
        print(f"Looking up task: {task}")
        table_name = get_dataset_table_name(task)

        if table_name is None:
            print(f"\t✗ Task '{task}' not found in task_registry")
            failed_tasks.append(task)
        else:
            record_count = get_record_count(table_name)
            print(f"\t✓ Found table: {table_name} ({record_count} records)")
            task_names.append(task)
            table_names.append(table_name)
            record_counts.append(record_count)

    print("-" * 80)

    if failed_tasks:
        print(f"\nError: {len(failed_tasks)} task(s) not found in task_registry:")
        for task in failed_tasks:
            print(f"\t- {task}")
        print("\nExiting without making any changes.")
        return

    if not task_names:
        print("No tasks to refresh.")
        return

    # Request confirmation unless --force is used
    if not args.force:
        if not confirm_deletion(task_names, table_names, record_counts):
            print("Refresh cancelled.")
            return

    # Process deletions and re-registration
    deleted_count = 0
    unregistered_count = 0
    registered_count = 0

    print(f"\nProcessing {len(task_names)} task(s)...")
    print("-" * 80)

    for task, table_name in zip(task_names, table_names):
        print(f"Refreshing task: {task}")

        if args.dry_run:
            print(f"\t[DRY RUN] Would delete table: {table_name}")
            print(f"\t[DRY RUN] Would unregister task: {task}")
            if not args.skip_register:
                print(f"\t[DRY RUN] Would execute register workflow")
            deleted_count += 1
            unregistered_count += 1
            if not args.skip_register:
                registered_count += 1
        else:
            # Delete dataset table
            if delete_dataset_table(table_name):
                print(f"\t✓ Deleted table: {table_name}")
                deleted_count += 1
            else:
                print(f"\t✗ Failed to delete table: {table_name}")

            # Unregister task
            if unregister_task(task):
                print(f"\t✓ Unregistered task from registry: {task}")
                unregistered_count += 1
            else:
                print(f"\t✗ Failed to unregister task: {task}")

            # Re-register task
            if not args.skip_register:
                print(f"\tStarting register workflow...")
                time.sleep(0.25)
                if execute_register_task(
                    task,
                    execution_timeout=args.register_timeout,
                    task_timeout=args.register_timeout,
                ):
                    print(f"\t✓ Register workflow triggered")
                    registered_count += 1
                else:
                    print(f"\t✗ Failed to trigger register workflow")

        time.sleep(0.25)

    print("-" * 80)

    # Summary
    print("\nRefresh Summary:")
    print("-" * 80)
    print(f"Tasks processed:           {len(task_names):5}")
    print(f"Tables deleted:            {deleted_count:5}")
    print(f"Tasks unregistered:        {unregistered_count:5}")
    if not args.skip_register:
        print(f"Register workflows triggered: {registered_count:5}")
    print("-" * 80)

    if args.dry_run:
        print("\n[DRY RUN MODE] - No actual changes were made")


# Example usage:
if __name__ == "__main__":
    main()
