from bson import ObjectId
from temporalio import activity
from temporalio.exceptions import ApplicationError

from app.app import get_app_logger, get_app_config
from packages.python.protocol.protocol import PocketNetworkEvaluationTaskRequest, PocketNetworkMongoDBResultSignature, SignatureSample
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.common.auto_heartbeater import auto_heartbeater

from packages.python.lmeh.utils.tokenizers import load_tokenizer, prepare_tokenizer

import json
from hashlib import sha256


@activity.defn
@auto_heartbeater
async def tokenizer_evaluate(args: PocketNetworkEvaluationTaskRequest) -> bool:
    """
    Returns a dict where each key is a task name with the evaluation result.
    :param args:
    :return:
    """

    
    app_config = get_app_config()
    eval_logger = get_app_logger("evaluation")
    config = app_config['config']

    try:
        args.task_id = ObjectId(args.task_id)
    except Exception as e:
        raise ApplicationError(
            "Bad Task ID format",
            str(e), args.task_id,
            type="BadParams",
            non_retryable=True,
        )

    mongo_client = config["mongo_client"]
    mongo_operator = MongoOperator(client=mongo_client)

    # Retrieve Task request.
    task_data = await mongo_operator.get_task(args.task_id)

    # Retrieve all responses
    responses = await mongo_operator.retrieve_responses(args.task_id)
    if len(responses)!=1:
        eval_logger.error(f"Found {len(responses)} responses, only 1 is expected.")
        raise ApplicationError(
                f"Task ID {args.task_id}: Found {len(responses)} responses, only 1 is expected.",
                args.task_id,
                type="ResponseError",
                non_retryable=False,
            )
    
    # Create the result, empty for now
    result = PocketNetworkMongoDBResultSignature(
        task_id =  args.task_id,
        num_samples =    0,
        signatures=[]
    )
    
    # Get tokenizer jsons
    tokenizer_jsons = json.loads(responses[0]['response']['response'])
    eval_logger.debug("Tokenizer found.", tokenizer_keys=list(tokenizer_jsons.keys()))
    tokenizer_ok = False
    if 'model_max_length' in tokenizer_jsons['tokenizer_config']:
        tokenizer_jsons['tokenizer_config']['model_max_length'] = int(
            tokenizer_jsons['tokenizer_config']['model_max_length'])
    try:
        # Try to load, if this succeds, the tokenizer is OK
        tokenizer = load_tokenizer(
                tokenizer_objects=tokenizer_jsons,
                wf_id='',
                tokenizer_ephimeral_path='/tmp/lala'
            )
        eval_logger.debug("Tokenizer loaded.")
        # This creates the structure used in the database, containing the hash
        tokenizer_mongo_new = prepare_tokenizer(tokenizer)
        eval_logger.debug("Tokenizer processed.")
        tokenizer_ok = True
    except Exception as e:
        # This is not an error is just a failure in retrieval of tokenizer
        eval_logger.info(f"Cannot load tokenizer from response.")
        eval_logger.error(f"Exeption:", Exeption=str(e))

        asdasdasd

        tokenizer_ok = False


    tokenizer_new = False
    if tokenizer_ok:
        # check if the tokenizer exists in db
        tokenizer_db = mongo_operator.get_tokenizer_entry(tokenizer_mongo_new['hash'])
        if tokenizer_db == None:
            # the tokenizer is not tracked, we need to create an entry
            tokenizer_new = True
            try:
                async with mongo_client.start_transaction() as session:
                    await mongo_client.db['tokenizers'].insert_many(
                                [tokenizer_mongo_new.model_dump(by_alias=True)],
                                ordered=False,
                                session=session,
                            )
            except Exception as e:
                eval_logger.error("Failed to save Tokenizer to MongoDB.")
                eval_logger.error(f"Exeption:", Exeption=str(e))
                raise ApplicationError("Failed to save tokenizer to MongoDB.", non_retryable=True)
            
        # Update the result with valid data
        result.num_samples =    1, # Always one
        result.signatures=[SignatureSample(
                signature = tokenizer_mongo_new['hash'],
                id = 0 # This task has a single sample id
            )]
        

        # Save to results db
        try:
            async with mongo_client.start_transaction() as session:
                await mongo_client.db['responses'].insert_many(
                            [result.model_dump(by_alias=True)],
                            ordered=False,
                            session=session,
                        )
        except Exception as e:
            eval_logger.error("Failed to save Result to MongoDB.")
            eval_logger.error(f"Exeption:", Exeption=str(e))
            raise ApplicationError("Failed to save result to MongoDB.", non_retryable=True)
    
    return {'tokenizer_is_valid': tokenizer_ok, 'tokenizer_is_new' : tokenizer_new}
