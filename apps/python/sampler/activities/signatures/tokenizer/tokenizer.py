from typing import List

from packages.python.protocol.protocol import (
    PocketNetworkMongoDBInstance,
    PocketNetworkMongoDBPrompt,
    PocketNetworkMongoDBTask,
    RequesterArgs,
)

def get_tokenizer_task(
    args: RequesterArgs,
) -> tuple[PocketNetworkMongoDBTask, List[PocketNetworkMongoDBInstance], List[PocketNetworkMongoDBPrompt]]:

    # Set call variables
    args.method = "GET"
    args.path = "/pokt/tokenizer"
    args.headers = {}

    # Create task
    task = PocketNetworkMongoDBTask(
        framework="signatures",
        requester_args=args,
        blacklist=[],
        qty=1,  # Tokenizer is hardcoded to this, no point in asking twice
        tasks="tokenizer",
        total_instances=1,
        request_type="",  # Remove
    )
    # There is a single instance for getting the tokenizer
    instance = PocketNetworkMongoDBInstance(task_id=task.id)
    # Create the void prompt
    prompt = PocketNetworkMongoDBPrompt(model_config={}, data="", task_id=task.id, instance_id=instance.id, timeout=10)

    return task, [instance], [prompt]
