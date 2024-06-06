import os
import sys

from app.app import get_app_config, get_app_logger
from temporalio import activity
from temporalio.exceptions import ApplicationError

# add file path to sys.path
sys.path.append(os.path.dirname(os.path.realpath(__file__)))
from activities.signatures.tokenizer.tokenizer import get_tokenizer_task
from activities.utils import auto_heartbeater

# Custom modules
from protocol.protocol import PocketNetworkTaskRequest


@activity.defn
@auto_heartbeater
async def sign_sample(args: PocketNetworkTaskRequest) -> bool:

    ############################################################
    # Config App
    ############################################################
    app_config = get_app_config()
    logger = get_app_logger("signatures")

    wf_id = activity.info().workflow_id

    config = get_app_config()["config"]
    logger.debug(
        f"Starting activity sign_sample:",
        wf_id=wf_id,
        task_name=args.tasks,
        address=args.requester_args.address,
        blacklist=args.blacklist,
        qty=args.qty,
    )
    args.mongodb_uri = config["mongodb_uri"]
    mongo_client = config["mongo_client"]
    try:
        # The ping command is cheap and does not require auth.
        mongo_client.admin.command("ping")
    except Exception as e:
        logger.error(f"Mongo DB connection failed.")
        raise ApplicationError("Mongo DB connection failed.", non_retryable=True)

    ############################################################
    # Gather all tasks
    ############################################################
    if args.tasks == "tokenizer":
        logger.debug(f"starting tokenizer task sample")
        task, instances, prompts = get_tokenizer_task(args.requester_args)

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
    logger.debug(f"Task:", task=task)
    # Instances
    for instance_mongo in instances:
        insert_mongo_instances.append(instance_mongo.model_dump(by_alias=True))
        logger.debug(f"Instance:", instance=instance_mongo)
        # Prompts
        for prompt_mongo in prompts:
            insert_mongo_prompt.append(prompt_mongo.model_dump(by_alias=True))
            logger.debug(f"Prompt:", PocketNetworkMongoDBPrompt=prompt_mongo)
    try:
        with mongo_client.start_session() as session:
            with session.start_transaction():
                mongo_client["pocket-ml-testbench"]["tasks"].insert_many(
                    insert_mongo_tasks, ordered=False, session=session
                )
                mongo_client["pocket-ml-testbench"]["instances"].insert_many(
                    insert_mongo_instances, ordered=False, session=session
                )
                mongo_client["pocket-ml-testbench"]["prompts"].insert_many(
                    insert_mongo_prompt, ordered=False, session=session
                )
                logger.debug("Instances saved to MongoDB successfully.")
    except Exception as e:
        logger.error("Failed to save Instances to MongoDB.")
        raise ApplicationError("Failed to save instances to MongoDB.", non_retryable=True)

    return True
