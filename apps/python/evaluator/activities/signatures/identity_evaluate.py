import json
from typing import Tuple
from app.app import get_app_config, get_app_logger
from bson import ObjectId
from temporalio import activity
import hashlib


from packages.python.common.auto_heartbeater import auto_heartbeater
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.protocol.protocol import (
    PocketNetworkEvaluationTaskRequest,
    PocketNetworkMongoDBResultSignature,
    SignatureSample,
    PocketNetworkMongoDBResultBase,
)


def calculate_hash(input_string):
    # Create a new SHA-256 hash object
    hash_object = hashlib.sha256()

    # Update the hash object with the input string (encode to bytes)
    hash_object.update(input_string.encode("utf-8"))

    # Get the hexadecimal representation of the hash
    hash_hex = hash_object.hexdigest()

    return hash_hex


@activity.defn
@auto_heartbeater
async def identity_evaluate(
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
            # task_id_str = args.task_id
            args.task_id = ObjectId(args.task_id)
        except Exception as e:
            eval_logger.error("bad Task ID format", error=str(e), task=args.task_id)
            return False, f"Bad Task ID format: {str(e)}"

        # Retrieve all responses
        responses = await mongo_operator.retrieve_responses(args.task_id)

        # Create the result, empty for now
        result = PocketNetworkMongoDBResultSignature(
            result_data=PocketNetworkMongoDBResultBase(
                task_id=args.task_id,
                status=0,
                num_samples=0,
                result_height=responses[0]["response"]["height"],
            ),
            signatures=[],
        )

        # Get responses hashes
        for this_response in responses:
            try:
                if this_response["response"]["error_code"] == 0:
                    # Decode model response
                    decoded_response = json.loads(this_response["response"]["response"])
                    # Calculate the response hash
                    hash_result = calculate_hash(decoded_response["choices"][0]["text"])
                else:
                    hash_result = ""
                # Append to signature list
                result.signatures.append(
                    SignatureSample(
                        signature=hash_result,
                        id=0,
                        status_code=this_response["response"]["error_code"],
                        error_str=this_response["response"]["error"],
                    )
                )
            except Exception:
                # Append a failed decode result
                result.signatures.append(
                    SignatureSample(
                        signature="",
                        id=0,
                        status_code=1,
                        error_str="failed to decode model response",
                    )
                )
            # Count samples included
            result.result_data.num_samples += 1

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

    except Exception as e:
        # Create a failed result
        result = PocketNetworkMongoDBResultSignature(
            result_data=PocketNetworkMongoDBResultBase(
                task_id=args.task_id,
                status=11,  # We failed to process
                num_samples=0,
                result_height=-1,
            ),
            signatures=[],
        )
        # TODO : This should not be part of the "find_one_and_update"
        try:
            result.pop("_id", None)
        except Exception as e:
            eval_logger.warn(
                "Cannot pop _id from config",
                task=args.task_id,
                error=str(e),
            )
            pass
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

        # Original error
        error_msg = "Failed to process evaluation."
        eval_logger.error(
            error_msg,
            task=args.task_id,
            error=str(e),
        )
        return False, f"{error_msg}: {str(e)}"

    return True, "OK"
