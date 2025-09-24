from typing import List

from packages.python.protocol.protocol import (
    PocketNetworkMongoDBInstance,
    CompletionRequest,
    PocketNetworkMongoDBPrompt,
    PocketNetworkMongoDBTask,
    RequesterArgs,
)

from activities.signatures.identity.prompt_gen import prompt_setup, get_prompt
import json


IDENTITY_SIGNATURE_COUNT = 2


def get_identity_task(
    args: RequesterArgs,
) -> tuple[
    PocketNetworkMongoDBTask,
    List[PocketNetworkMongoDBInstance],
    List[PocketNetworkMongoDBPrompt],
]:
    # Set call variables
    args.method = "POST"
    args.path = "/v1/completions"
    args.headers = {}

    # Create task
    task = PocketNetworkMongoDBTask(
        framework="signatures",
        requester_args=args,
        blacklist=[],
        qty=IDENTITY_SIGNATURE_COUNT,
        tasks="identity",
        total_instances=IDENTITY_SIGNATURE_COUNT, # one instance per prompt
        request_type="",  # Remove
    )
    # Create each prompt:
    instances = list()
    prompts = list()
    prompt_setup()
    for prompt_idx in range(IDENTITY_SIGNATURE_COUNT):

        instance = PocketNetworkMongoDBInstance(task_id=task.id)
        # Create the request
        request = CompletionRequest(
                model="pocket_network",
                prompt=get_prompt(),
                max_tokens=100,
                temperature=0.0,
                seed=prompt_idx,
            )
        # Create the prompt
        prompt = PocketNetworkMongoDBPrompt(
            model_config={}, 
            data=json.dumps(request.to_dict()), 
            task_id=task.id, 
            instance_id=instance.id, 
            timeout=60
        )

        instances.append(instance)
        prompts.append(prompt)

    return task, instances, prompts
