from temporalio import activity
from temporalio.exceptions import ApplicationError
from app.app import get_app_logger, get_app_config

import os
import sys

# add file path to sys.path
sys.path.append(os.path.dirname(os.path.realpath(__file__)))
# Custom modules
from protocol.protocol import PocketNetworkTaskRequest
from activities.signatures.tokenizer import get_tokenizer_task
from activities.utils import auto_heartbeater


@activity.defn
@auto_heartbeater
async def sample(args: PocketNetworkTaskRequest) -> bool:

    ############################################################
    # Config App
    ############################################################
    app_config = get_app_config()
    eval_logger = get_app_logger("sampling signatures")

    wf_id = activity.info().workflow_id
    
    config = get_app_config()['config']
    eval_logger.debug(f"Starting activity sample signatures:", task_name=args.tasks, address=args.requester_args.address,
                        blacklist=args.blacklist, qty=args.qty)
    args.mongodb_uri = config["mongodb_uri"]
    mongo_client = config["mongo_client"]
    try:
        # The ping command is cheap and does not require auth.
        mongo_client.admin.command('ping')
    except Exception as e:
        eval_logger.error(f"Mongo DB connection failed.")
        raise ApplicationError("Mongo DB connection failed.", non_retryable=True)
    if args.llm_args is None:
        args.llm_args = {}

    ############################################################
    # Gather all tasks
    ############################################################
    if args.tasks == 'tokenizer':
        eval_logger.debug(f"starting tokenizer task sample")
        eval_task, eval_instances, eval_prompts = get_tokenizer_task(args.requester_args)
        
    else:
        eval_logger.error(f"requested task {args.tasks} is not supported")
        return False

    ############################################################
    # Insert into Mongo
    ############################################################
    
    insert_mongo_prompt = []
    insert_mongo_tasks = []
    insert_mongo_instances = []
    insert_mongo_tasks.append(eval_task.model_dump(by_alias=True))
    # Instances
    for instance_mongo in eval_instances:
        insert_mongo_instances.append(instance_mongo)
        eval_logger.debug(f"Instance:", instance=instance_mongo)
        # Prompts
        for prompt_mongo in eval_prompts:
            insert_mongo_prompt.append(prompt_mongo.model_dump())
            eval_logger.debug(f"Data:", PocketNetworkMongoDBPrompt=prompt_mongo)
    try:
        with mongo_client.start_session() as session:
            with session.start_transaction():
                mongo_client['pocket-ml-testbench']['tasks'].insert_many(insert_mongo_tasks, ordered=False,
                                                                         session=session)
                mongo_client['pocket-ml-testbench']['instances'].insert_many(insert_mongo_instances, ordered=False,
                                                                             session=session)
                mongo_client['pocket-ml-testbench']['prompts'].insert_many(insert_mongo_prompt, ordered=False,
                                                                           session=session)
                eval_logger.debug("Instances saved to MongoDB successfully.")
    except Exception as e:
        eval_logger.error("Failed to save Instances to MongoDB.")
        raise ApplicationError("Failed to save instances to MongoDB.", error=e, non_retryable=True)

    return True