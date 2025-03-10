import os
import sys

from app.app import get_app_config, get_app_logger
from temporalio import activity
from temporalio.exceptions import ApplicationError

# add file path to sys.path
sys.path.append(os.path.dirname(os.path.realpath(__file__)))
from activities.signatures.tokenizer.tokenizer import get_tokenizer_task
from activities.signatures.config.config import get_config_task
from packages.python.common.auto_heartbeater import auto_heartbeater

# Custom modules
from packages.python.protocol.protocol import PocketNetworkTaskRequest


@activity.defn
@auto_heartbeater
async def sign_sample(args: PocketNetworkTaskRequest) -> bool:
    ############################################################
    # Config App
    ############################################################
    # app_config = get_app_config()
    logger = get_app_logger("signatures")

    wf_id = activity.info().workflow_id

    config = get_app_config()["config"]
    logger.debug(
        "Starting activity sign_sample:",
        wf_id=wf_id,
        task_name=args.tasks,
        address=args.requester_args.address,
        blacklist=args.blacklist,
        qty=args.qty,
    )
    mongo_client = config["mongo_client"]
    ############################################################
    # Gather all tasks
    ############################################################
    if args.tasks == "tokenizer":
        logger.debug("starting tokenizer task sample")
        task, instances, prompts = get_tokenizer_task(args.requester_args)

    elif args.tasks == "config":
        logger.debug("starting config task sample")
        task, instances, prompts = get_config_task(args.requester_args)

    else:
        logger.error(f"requested task {args.tasks} is not supported")
        return False

    ############################################################
    # Insert into Mongo
    ############################################################

    insert_mongo_tasks = []
    insert_mongo_prompt = []
    insert_mongo_instances = []
    insert_mongo_tasks.append(task.model_dump(by_alias=True))
    logger.debug("Task:", task=task)
    # Instances
    for instance_mongo in instances:
        insert_mongo_instances.append(instance_mongo.model_dump(by_alias=True))
        logger.debug("Instance:", instance=instance_mongo)
        # Prompts
        for prompt_mongo in prompts:
            insert_mongo_prompt.append(prompt_mongo.model_dump(by_alias=True))
            logger.debug("Prompt:", PocketNetworkMongoDBPrompt=prompt_mongo)
    try:
        async with mongo_client.start_transaction() as session:
            await mongo_client.db["tasks"].insert_many(
                insert_mongo_tasks,
                ordered=False,
                session=session,
            )
            await mongo_client.db["instances"].insert_many(
                insert_mongo_instances,
                ordered=False,
                session=session,
            )
            await mongo_client.db["prompts"].insert_many(
                insert_mongo_prompt,
                ordered=False,
                session=session,
            )

            logger.debug("Instances saved to MongoDB successfully.")
    except Exception as e:
        logger.error("Failed to save Instances to MongoDB.")
        logger.error("Exeption:", Exeption=str(e))
        raise ApplicationError(
            "Failed to save instances to MongoDB.", non_retryable=True
        )

    return True
