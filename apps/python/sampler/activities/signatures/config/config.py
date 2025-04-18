from typing import List

from packages.python.protocol.protocol import (
    PocketNetworkMongoDBInstance,
    PocketNetworkMongoDBPrompt,
    PocketNetworkMongoDBTask,
    RequesterArgs,
)


def get_config_task(
    args: RequesterArgs,
) -> tuple[
    PocketNetworkMongoDBTask,
    List[PocketNetworkMongoDBInstance],
    List[PocketNetworkMongoDBPrompt],
]:
    # Set call variables
    args.method = "GET"
    args.path = "/pokt/config"
    args.headers = {}

    # Create task
    task = PocketNetworkMongoDBTask(
        framework="signatures",
        requester_args=args,
        blacklist=[],
        qty=1,  # Config is hardcoded to this, no point in asking twice
        tasks="config",
        total_instances=1,
        request_type="",  # Remove
    )
    # There is a single instance for getting the tokenizer
    instance = PocketNetworkMongoDBInstance(task_id=task.id)
    # Create the void prompt
    prompt = PocketNetworkMongoDBPrompt(
        model_config={}, data="", task_id=task.id, instance_id=instance.id, timeout=60
    )

    return task, [instance], [prompt]
