import json
from datetime import datetime
from typing import Tuple
from app.app import get_app_config, get_app_logger
from bson import ObjectId
from temporalio import activity

from packages.python.common.auto_heartbeater import auto_heartbeater
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.lmeh.utils.tokenizers import (
    load_tokenizer,
    prepare_tokenizer,
)
from packages.python.protocol.protocol import (
    PocketNetworkEvaluationTaskRequest,
    PocketNetworkMongoDBResultSignature,
    PocketNetworkMongoDBTokenizer,
    SignatureSample,
    PocketNetworkMongoDBResultBase,
)


@activity.defn
@auto_heartbeater
async def tokenizer_evaluate(
    args: PocketNetworkEvaluationTaskRequest,
) -> Tuple[bool, str]:
    """
    Returns a dict where each key is a task name with the evaluation result.
    :param args:
    :return:
    """
    app_config = get_app_config()
    eval_logger = get_app_logger("evaluation")
    config = app_config["config"]
    mongo_client = config["mongo_client"]
    mongo_operator = MongoOperator(client=mongo_client)

    try:
        try:
            task_id_str = args.task_id
            args.task_id = ObjectId(args.task_id)
        except Exception as e:
            eval_logger.error("bad Task ID format", error=str(e), task=args.task_id)
            return False, f"Bad Task ID format: {str(e)}"
            # raise ApplicationError(
            #     "Bad Task ID format",
            #     str(e),
            #     args.task_id,
            #     type="BadParams",
            #     non_retryable=True,
            # )

        # Retrieve all responses
        responses = await mongo_operator.retrieve_responses(args.task_id)
        if len(responses) != 1:
            # This should not be fatal
            eval_logger.warn(
                f"Task ID {args.task_id}: Found {len(responses)} responses, only 1 is expected. Using the first one and proceeding."
            )
            # raise ApplicationError(
            #     f"Task ID {args.task_id}: Found {len(responses)} responses, only 1 is expected.",
            #     str(args.task_id),
            #     type="ResponseError",
            #     non_retryable=False,
            # )

        # Create the result, empty for now
        result = PocketNetworkMongoDBResultSignature(
            result_data=PocketNetworkMongoDBResultBase(
                task_id=args.task_id,
                status=responses[0]["response"]["error_code"],
                num_samples=0,
                result_height=responses[0]["response"]["height"],
                result_time=datetime.today().isoformat(),
            ),
            signatures=[],
        )

        # Get tokenizer jsons
        tokenizer_decoded = False
        try:
            tokenizer_jsons = json.loads(responses[0]["response"]["response"])
            tokenizer_decoded = True
        except Exception as e:
            eval_logger.debug("Exeption:", Exeption=str(e))
            # Update the result with errored data
            result.result_data.num_samples = 1  # Always one
            result.result_data.status = (
                0  # OK, proceed to add to signatures buffer (by manager)
            )
            result.signatures = [
                SignatureSample(
                    signature="Cannot decode tokenizer",
                    id=0,
                    status_code=11,  # Error at evaluation
                    error_str=str(e),
                )  # This task has a single sample id
            ]

        tokenizer_ok = False
        if tokenizer_decoded:
            eval_logger.debug(
                "Tokenizer found.", tokenizer_keys=list(tokenizer_jsons.keys())
            )

            if "model_max_length" in tokenizer_jsons["tokenizer_config"]:
                tokenizer_jsons["tokenizer_config"]["model_max_length"] = int(
                    tokenizer_jsons["tokenizer_config"]["model_max_length"]
                )
            try:
                # Try to load, if this succeds, the tokenizer is OK
                temp_path = "/tmp/" + task_id_str
                tokenizer = load_tokenizer(
                    tokenizer_objects=tokenizer_jsons,
                    wf_id="",
                    tokenizer_ephimeral_path=temp_path,
                )
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
                # Update the result with errored data
                result.result_data.num_samples = 1  # Always one
                result.result_data.status = (
                    0  # OK, proceed to add to signatures buffer (by manager)
                )
                result.signatures = [
                    SignatureSample(
                        signature="Cannot load tokenizer from decoded data",
                        id=0,
                        status_code=11,  # Error at evaluation
                        error_str=str(e),
                    )  # This task has a single sample id
                ]

        tokenizer_new = False
        if tokenizer_ok:
            # check if the tokenizer exists in db
            tokenizer_db = await mongo_operator.get_tokenizer_entry(
                tokenizer_mongo_new.hash
            )
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
                    error_msg = "Failed to save model Tokenizer to MongoDB."
                    eval_logger.error(
                        error_msg,
                        task=args.task_id,
                        error=str(e),
                    )
                    return False, f"{error_msg}: {str(e)}"
                    # eval_logger.error("Failed to save Tokenizer to MongoDB.")
                    # eval_logger.error("Exeption:", Exeption=str(e))
                    # raise ApplicationError(
                    #     "Failed to save tokenizer to MongoDB.", non_retryable=True
                    # )

            # Update the result with valid data
            result.result_data.num_samples = 1  # Always one
            result.result_data.status = 0  # OK
            result.signatures = [
                SignatureSample(
                    signature=str(tokenizer_mongo_new.hash),
                    id=0,
                    status_code=0,
                    error_str="",
                )  # This task has a single sample id
            ]

        # Save to results db (a failure is also an answer)
        try:
            async with mongo_client.start_transaction() as session:
                await mongo_client.db["results"].find_one_and_update(
                    {"result_data.task_id": args.task_id},
                    {"$set": result.model_dump(by_alias=True)},
                    upsert=True,
                    session=session,
                )
                await mongo_client.db["tasks"].update_one(
                    {"_id": args.task_id},
                    {"$set": {"evaluated": True}},
                    session=session,
                )
            eval_logger.debug("Saved result to DB.")
        except Exception as e:
            error_msg = "Failed to save Result to MongoDB. (correct evaluation path)"
            eval_logger.error(
                error_msg,
                task=args.task_id,
                error=str(e),
            )
            return False, f"{error_msg}: {str(e)}"
            # eval_logger.error("Failed to save Result to MongoDB.")
            # eval_logger.error("Exception:", Exeption=str(e))
            # raise ApplicationError(
            #     "Failed to save result to MongoDB.", non_retryable=True
            # )

        eval_logger.info(
            "Tokenizer Status:",
            tokenizer_decoded=tokenizer_decoded,
            tokenizer_is_valid=tokenizer_ok,
            tokenizer_is_new=tokenizer_new,
        )
    except Exception as e:
        # Create a failed result
        result = PocketNetworkMongoDBResultSignature(
            result_data=PocketNetworkMongoDBResultBase(
                task_id=args.task_id,
                status=11,  # We failed to process
                num_samples=0,
                result_height=-1,
                result_time=datetime.today().isoformat(),
            ),
            signatures=[],
        )
        # Save to results db (a failure is also an answer)
        try:
            async with mongo_client.start_transaction() as session:
                await mongo_client.db["results"].find_one_and_update(
                    {"result_data.task_id": args.task_id},
                    {"$set": result.model_dump(by_alias=True)},
                    upsert=True,
                    session=session,
                )
                await mongo_client.db["tasks"].update_one(
                    {"_id": args.task_id},
                    {"$set": {"evaluated": True}},
                    session=session,
                )
            eval_logger.debug("Saved result to DB.")
        except Exception as e:
            error_msg = "Failed to save Result to MongoDB. (failed evaluation path)"
            eval_logger.error(
                error_msg,
                task=args.task_id,
                error=str(e),
            )
            return False, f"{error_msg}: {str(e)}"
            # eval_logger.error("Failed to save Result to MongoDB.")
            # eval_logger.error("Exception:", Exeption=str(e))
            # raise ApplicationError(
            #     "Failed to save result to MongoDB.", non_retryable=True
            # )
        # Original error
        error_msg = "Failed to process evaluation."
        eval_logger.error(
            error_msg,
            task=args.task_id,
            error=str(e),
        )
        return False, f"{error_msg}: {str(e)}"

    return True, "OK"
