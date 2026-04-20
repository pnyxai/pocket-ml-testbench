import subprocess
import argparse


# Default configuration
DEPLOYMENT_NAME = "deploy/temporal-admintools"
TEMPORAL_NAMESPACE = "pocket-ml-testbench"

BASE_COMMAND = ["kubectl", "exec", "-it"]


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
        result = subprocess.run(command, check=True, capture_output=True, text=True)
        return result.stdout
    except subprocess.CalledProcessError as e:
        print(f"Error executing command: {e}")
        return ""


def fetch_schedule_list(temporal_namespace):
    """
    Fetch the list of schedules by executing temporal schedule list command.
    Returns a list of schedule IDs.
    """
    command = BASE_COMMAND + [
        "--",
        "temporal",
        "schedule",
        "list",
        "--namespace",
        temporal_namespace,
    ]

    output = run_command_with_output(command)
    if not output:
        print("Failed to fetch schedule list.")
        return []

    # Parse the output to extract schedule IDs
    schedules = []
    for line in output.split("\n"):
        line = line.strip()
        if not line:
            continue
        # Extract the schedule ID (first column before any whitespace followed by JSON)
        parts = line.split()
        if parts and "{" in line:
            # Extract schedule ID (everything before the JSON part)
            schedule_id = line.split("{")[0].strip()
            if schedule_id:
                schedules.append(schedule_id)

    return schedules


def filter_schedules(schedules, match_filter):
    """Filter schedules by substring match."""
    if not match_filter:
        return schedules

    filtered = [s for s in schedules if match_filter.lower() in s.lower()]
    return filtered


def confirm_deletion(schedules, match_filter=None):
    """
    Display schedules to be deleted and ask for confirmation.
    Returns True if user confirms, False otherwise.
    """
    if not schedules:
        print("No schedules to delete.")
        return False

    print("\nSchedules to be deleted:")
    print("-" * 60)
    for schedule in schedules:
        print(f"  {schedule}")
    print("-" * 60)
    print(f"Total: {len(schedules)} schedule(s)")

    if match_filter:
        print(f"Filter applied: '{match_filter}'")

    while True:
        response = input("\nProceed with deletion? (yes/no): ").strip().lower()
        if response in ("yes", "y"):
            return True
        elif response in ("no", "n"):
            return False
        else:
            print("Please enter 'yes' or 'no'.")


def delete_schedules(schedules, temporal_namespace, dry_run=False):
    """Delete the specified schedules."""
    deleted_count = 0
    failed_count = 0

    for schedule in schedules:
        command = BASE_COMMAND + [
            "--",
            "temporal",
            "schedule",
            "delete",
            "--schedule-id",
            schedule,
            "--namespace",
            temporal_namespace,
        ]

        if dry_run:
            print(f"[DRY RUN] Would delete: {schedule}")
            deleted_count += 1
        else:
            print(f"Deleting: {schedule}")
            if run_command(command):
                deleted_count += 1
            else:
                failed_count += 1

    print(f"\nDeletion complete: {deleted_count} deleted, {failed_count} failed")


def main():
    global DEPLOYMENT_NAME, TEMPORAL_NAMESPACE, BASE_COMMAND

    parser = argparse.ArgumentParser(
        description="Delete Temporal schedules with optional filtering."
    )
    parser.add_argument(
        "--match",
        help="Filter schedules by substring match (case-insensitive)",
    )
    parser.add_argument(
        "--k8s-namespace",
        help="Kubernetes namespace (default: not set)",
    )
    parser.add_argument(
        "--k8s-deployment",
        help=f"Kubernetes deployment name (default: {DEPLOYMENT_NAME})",
    )
    parser.add_argument(
        "--temporal-namespace",
        help=f"Temporal namespace (default: {TEMPORAL_NAMESPACE})",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show what would be deleted without actually deleting",
    )
    parser.add_argument(
        "--force",
        action="store_true",
        help="Skip confirmation prompt and delete immediately",
    )

    args = parser.parse_args()

    # Apply configuration overrides
    if args.temporal_namespace:
        TEMPORAL_NAMESPACE = args.temporal_namespace

    if args.k8s_deployment:
        print(f"Using k8s Namespace: {args.k8s_namespace}")
        DEPLOYMENT_NAME = args.k8s_deployment
    BASE_COMMAND += [DEPLOYMENT_NAME]

    if args.k8s_namespace:
        print(f"Using k8s Namespace: {args.k8s_namespace}")
        BASE_COMMAND += ["-n", f"{args.k8s_namespace}"]

    print(f"Fetching schedules from {TEMPORAL_NAMESPACE}...")
    schedules = fetch_schedule_list(TEMPORAL_NAMESPACE)

    if not schedules:
        print("No schedules found.")
        return

    print(f"Found {len(schedules)} schedule(s).")

    # Apply filter if provided
    if args.match:
        print(f"Applying filter: '{args.match}'")
        schedules = filter_schedules(schedules, args.match)
        if not schedules:
            print(f"Warning: No schedules match the filter '{args.match}'")
            return

    print(f"After filtering: {len(schedules)} schedule(s)")

    # Request confirmation unless --force is used
    if not args.force:
        if not confirm_deletion(schedules, args.match):
            print("Deletion cancelled.")
            return

    # Delete the schedules
    delete_schedules(schedules, TEMPORAL_NAMESPACE, args.dry_run)


if __name__ == "__main__":
    main()
