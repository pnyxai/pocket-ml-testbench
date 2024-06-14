import json
from hashlib import sha256

from app.app import get_app_config, get_app_logger
from bson import ObjectId
from temporalio import activity
from temporalio.exceptions import ApplicationError

from packages.python.common.auto_heartbeater import auto_heartbeater
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.lmeh.utils.tokenizers import load_tokenizer, prepare_tokenizer
from packages.python.protocol.protocol import (
    PocketNetworkEvaluationTaskRequest,
    PocketNetworkMongoDBResultSignature,
    PocketNetworkMongoDBTokenizer,
    SignatureSample,
)


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
    config = app_config["config"]

    try:
        task_id_str = args.task_id
        args.task_id = ObjectId(args.task_id)
    except Exception as e:
        raise ApplicationError(
            "Bad Task ID format",
            str(e),
            args.task_id,
            type="BadParams",
            non_retryable=True,
        )

    mongo_client = config["mongo_client"]
    mongo_operator = MongoOperator(client=mongo_client)

    # Retrieve Task request.
    task_data = await mongo_operator.get_task(args.task_id)

    # Retrieve all responses
    responses = await mongo_operator.retrieve_responses(args.task_id)
    if len(responses) != 1:
        eval_logger.error(f"Found {len(responses)} responses, only 1 is expected.")
        raise ApplicationError(
            f"Task ID {args.task_id}: Found {len(responses)} responses, only 1 is expected.",
            args.task_id,
            type="ResponseError",
            non_retryable=False,
        )

    # Create the result, empty for now
    result = PocketNetworkMongoDBResultSignature(task_id=args.task_id, num_samples=0, signatures=[])

    # Get tokenizer jsons
    tokenizer_decoded = False
    try:
        tokenizer_jsons = json.loads(responses[0]["response"]["response"])
        tokenizer_decoded = True
    except Exception as e:
        eval_logger.debug(f"Exeption:", Exeption=str(e))

    tokenizer_ok = False
    if tokenizer_decoded:
        eval_logger.debug("Tokenizer found.", tokenizer_keys=list(tokenizer_jsons.keys()))

        if "model_max_length" in tokenizer_jsons["tokenizer_config"]:
            tokenizer_jsons["tokenizer_config"]["model_max_length"] = int(
                tokenizer_jsons["tokenizer_config"]["model_max_length"]
            )
        try:
            # Try to load, if this succeds, the tokenizer is OK
            temp_path = "/tmp/" + task_id_str
            tokenizer = load_tokenizer(tokenizer_objects=tokenizer_jsons, wf_id="", tokenizer_ephimeral_path=temp_path)
            eval_logger.debug("Tokenizer loaded.")
            # This creates the structure used in the database, containing the hash
            tokenizer_jsons_loaded, tokenizer_hash_loaded = prepare_tokenizer(
                tokenizer, TOKENIZER_EPHIMERAL_PATH=temp_path
            )
            tokenizer_mongo_new = PocketNetworkMongoDBTokenizer(
                tokenizer=tokenizer_jsons_loaded, hash=tokenizer_hash_loaded
            )
            eval_logger.debug("Tokenizer processed.")
            tokenizer_ok = True
        except Exception as e:
            # This is not an error is just a failure in retrieval of tokenizer
            eval_logger.info("Cannot load tokenizer from response.")
            eval_logger.debug("Exeption:", Exeption=str(e))
            tokenizer_ok = False

    tokenizer_new = False
    if tokenizer_ok:
        # check if the tokenizer exists in db
        tokenizer_db = await mongo_operator.get_tokenizer_entry(tokenizer_mongo_new.hash)
        if tokenizer_db is None:
            eval_logger.debug("Tokenizer does not exists.")
            # the tokenizer is not tracked, we need to create an entry
            tokenizer_new = True
            try:
                async with mongo_client.start_transaction() as session:
                    await mongo_client.db["tokenizers"].insert_many(
                        [tokenizer_mongo_new.model_dump(by_alias=True)],
                        ordered=False,
                        session=session,
                    )
                eval_logger.debug("Saved new tokenizer to DB.")
            except Exception as e:
                eval_logger.error("Failed to save Tokenizer to MongoDB.")
                eval_logger.error("Exeption:", Exeption=str(e))
                raise ApplicationError("Failed to save tokenizer to MongoDB.", non_retryable=True)

        # Update the result with valid data
        result.num_samples = 1  # Always one
        result.signatures = [
            SignatureSample(signature=str(tokenizer_mongo_new.hash), id=0)  # This task has a single sample id
        ]

    # Save to results db (a failure is also an answer)
    try:
        async with mongo_client.start_transaction() as session:
            await mongo_client.db["results"].insert_many(
                [result.model_dump(by_alias=True)],
                ordered=False,
                session=session,
            )
        eval_logger.debug("Saved result to DB.")
    except Exception as e:
        eval_logger.error("Failed to save Result to MongoDB.")
        eval_logger.error(f"Exeption:", Exeption=str(e))
        raise ApplicationError("Failed to save result to MongoDB.", non_retryable=True)

    eval_logger.info(
        f"Status:", tokenizer_decoded=tokenizer_decoded, tokenizer_is_valid=tokenizer_ok, tokenizer_is_new=tokenizer_new
    )

    return True
