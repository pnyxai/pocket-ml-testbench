import json
from datetime import datetime

from app.app import get_app_config, get_app_logger
from bson import ObjectId
from temporalio import activity
from temporalio.exceptions import ApplicationError

from packages.python.common.auto_heartbeater import auto_heartbeater
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.lmeh.utils.tokenizers import (
    load_config,
    prepare_config,
)
from packages.python.protocol.protocol import (
    PocketNetworkEvaluationTaskRequest,
    PocketNetworkMongoDBResultSignature,
    PocketNetworkMongoDBConfig,
    SignatureSample,
    PocketNetworkMongoDBResultBase,
)


@activity.defn
@auto_heartbeater
async def model_config_evaluate(args: PocketNetworkEvaluationTaskRequest) -> bool:
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
            raise ApplicationError(
                "Bad Task ID format",
                str(e),
                args.task_id,
                type="BadParams",
                non_retryable=True,
            )

        # Retrieve all responses
        responses = await mongo_operator.retrieve_responses(args.task_id)
        if len(responses) != 1:
            # This should not be fatal
            eval_logger.warn(f"Task ID {args.task_id}: Found {len(responses)} responses, only 1 is expected.")
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

        # Get config jsons
        model_config_decoded = False
        try:
            model_config_jsons = json.loads(responses[0]["response"]["response"])
            model_config_decoded = True
        except Exception as e:
            eval_logger.debug("Exeption:", Exeption=str(e))
            # Update the result with errored data
            result.result_data.num_samples = 1  # Always one
            result.result_data.status = (
                0  # OK, proceed to add to signatures buffer (by manager)
            )
            result.signatures = [
                SignatureSample(
                    signature="Cannot decode configuration",
                    id=0,
                    status_code=11,  # Error at evaluation
                )  # This task has a single sample id
            ]

        model_config_ok = False
        if model_config_decoded:
            eval_logger.debug(
                "Model config found.", model_config_keys=list(model_config_jsons.keys())
            )

            try:
                # Try to load, if this succeds, the model config is OK
                temp_path = "/tmp/" + task_id_str

                _config = load_config(
                    config_objects=model_config_jsons,
                    wf_id="",
                    config_ephimeral_path=temp_path,
                    trust_remote_code=False,
                )
                eval_logger.debug("Config loaded.")
                # This creates the structure used in the database, containing the hash
                config_jsons_loaded, config_hash_loaded = prepare_config(
                    _config, CONFIG_EPHIMERAL_PATH=temp_path
                )
                model_config_mongo_new = PocketNetworkMongoDBConfig(
                    config=config_jsons_loaded, hash=config_hash_loaded
                )
                eval_logger.debug("Config processed.")

                model_config_ok = True
            except Exception as e:
                # This is not an error is just a failure in retrieval of the model config
                eval_logger.info("Cannot load the model config from response.")
                eval_logger.debug("Exeption:", Exeption=str(e))
                model_config_ok = False
                # Update the result with errored data
                result.result_data.num_samples = 1  # Always one
                result.result_data.status = (
                    0  # OK, proceed to add to signatures buffer (by manager)
                )
                result.signatures = [
                    SignatureSample(
                        signature="Cannot load model configuration",
                        id=0,
                        status_code=11,  # Error at evaluation
                    )  # This task has a single sample id
                ]

        model_config_new = False
        if model_config_ok:
            # check if the model_config exists in db
            model_config_db = await mongo_operator.get_config_entry(
                model_config_mongo_new.hash
            )
            if model_config_db is None:
                eval_logger.debug("Model config does not exists.")
                # the model config is not tracked, we need to create an entry
                model_config_new = True
                try:
                    async with mongo_client.start_transaction() as session:
                        await mongo_client.db["configs"].insert_many(
                            [model_config_mongo_new.model_dump(by_alias=True)],
                            ordered=False,
                            session=session,
                        )
                    eval_logger.debug("Saved new config to DB.")
                except Exception as e:
                    eval_logger.error("Failed to save model cofig to MongoDB.")
                    eval_logger.error("Exeption:", Exeption=str(e))
                    raise ApplicationError(
                        "Failed to save model config to MongoDB.", non_retryable=True
                    )

            # Update the result with valid data
            result.result_data.num_samples = 1  # Always one
            result.result_data.status = 0  # OK
            result.signatures = [
                SignatureSample(
                    signature=str(model_config_mongo_new.hash), id=0, status_code=0
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
            eval_logger.error("Failed to save Result to MongoDB.")
            eval_logger.error("Exception:", Exeption=str(e))
            raise ApplicationError(
                "Failed to save result to MongoDB.", non_retryable=True
            )

        eval_logger.info(
            "Model Config Status:",
            model_config_decoded=model_config_decoded,
            model_config_is_valid=model_config_ok,
            model_config_is_new=model_config_new,
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
        # This should not be part of the "find_one_and_update"
        result.pop("_id", None)
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
            eval_logger.error("Failed to save Result to MongoDB.")
            eval_logger.error("Exception:", Exeption=str(e))
            raise ApplicationError(
                "Failed to save result to MongoDB.", non_retryable=True
            )
        raise e

    return True
