from bson import ObjectId
from temporalio import activity
from temporalio.exceptions import ApplicationError
from typing import Tuple

from app.app import get_app_logger, get_app_config
from packages.python.lmeh.pocket_lm_eval.models.pocket_network import EvaluatorLM
from packages.python.lmeh.utils.common import get_task_manager
from packages.python.lmeh.utils import generator as lmeh_generator
from packages.python.lmeh.utils import task_config as open_llm_config
from packages.python.protocol.protocol import PocketNetworkEvaluationTaskRequest
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.common.auto_heartbeater import auto_heartbeater
from packages.python.lmeh.pocket_lm_eval.tasks import TASK_MANAGER_EVALUATE_STAGE
from packages.python.lmeh.utils import sql as lmeh_sql


@activity.defn
@auto_heartbeater
async def lmeh_evaluate(args: PocketNetworkEvaluationTaskRequest) -> Tuple[bool, str]:
    """
    Returns a dict where each key is a task name with the evaluation result.
    :param args:
    :return:
    """
    ############################################################
    # START: POCKET NETWORK CODE
    ############################################################
    app_config = get_app_config()
    eval_logger = get_app_logger("evaluation")
    config = get_app_config()["config"]
    mongo_client = config["mongo_client"]
    mongo_operator = MongoOperator(client=mongo_client)

    try:
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

    try:
        if args.llm_args is None:
            args.llm_args = {}
        eval_logger.info(
            "Starting activity lmeh_evaluate",
            task_id=str(args.task_id),
        )

        doc_ids = await mongo_operator.get_doc_ids_by_task(args.task_id)
        args.doc_ids = doc_ids

        # Recreate Task request.
        task_mongo = await mongo_operator.get_task(args.task_id)
        args.tasks = task_mongo.tasks
        args.blacklist = task_mongo.blacklist
        args.qty = task_mongo.qty
        args.requester_args = task_mongo.requester_args
        if task_mongo.gen_kwargs is not None:
            args.gen_kwargs = task_mongo.gen_kwargs
        if args.llm_args is None:
            args.llm_args = {}

        args.requester_args = task_mongo.requester_args
        if args.tasks is None:
            eval_logger.error("Need to specify task to evaluate.", task=args.task_id)
            return False, "Need to specify task to evaluate."
            # raise ApplicationError(
            #     "Need to specify task to evaluate.",
            #     args.tasks,
            #     type="BadParams",
            #     non_retryable=True,
            # )
        if not task_mongo.done:
            eval_logger.error("Task is not done.", task=args.task_id)
            return False, "Task is not done."
            # raise ApplicationError(
            #     "Task is not done.",
            #     args.task_id,
            #     type="TaskNotDone",
            #     non_retryable=False,
            # )
        ############################################################
        # END: POCKET NETWORK CODE
        ############################################################

        eval_logger.debug(
            "Starting activity evaluation:",
            task_id=args.task_id,
            address=args.requester_args.address,
            blacklist=args.blacklist,
            qty=args.qty,
        )

        # Check include path and override with config
        # TODO : This should not be an argument from the request
        include_path = args.include_path
        if "include_path" in config:
            include_path = config["include_path"]
            eval_logger.info(
                f"Using additional tasks from : {include_path}",
            )

        # retrieve database connection
        eval_logger.debug("Acquiring Postgres Connection from pool")
        async with app_config["postgres"].acquire() as conn:
            async with conn.transaction():
                task_manager, task_names = get_task_manager(
                    tasks=args.tasks,
                    include_path=include_path,
                    verbosity=str(args.verbosity),
                    logger=eval_logger,
                    postgres_conn=conn,
                    pocket_args=args,
                    stage=TASK_MANAGER_EVALUATE_STAGE,
                )
                eval_logger.debug("Read task names", task_names=task_names)

                for task_name in task_names:
                    # lookup the task on task_registry before try to load it
                    if not await lmeh_sql.checked_task(task_name, connection=conn):
                        eval_logger.error(
                            "Task not found on task_registry table.", task=args.task_id
                        )
                        return False, "Task not found on task_registry table."
                        # raise ApplicationError(
                        #     "Task not found on task_registry table",
                        #     task_name,
                        #     type="NotFound",
                        #     non_retryable=True,
                        # )

                    # generate configurable tasks
                    try:
                        open_llm_cfg = open_llm_config.get_task_config(task_names[0])
                        open_llm_filters = open_llm_cfg.get("filters", ["none"])
                        open_llm_metrics = open_llm_cfg["metrics"]
                        task_dict = lmeh_generator.get_configurable_task(
                            tasks=[task_name],
                            num_fewshot=args.num_fewshot,
                            check_integrity=False,
                            gen_kwargs=args.gen_kwargs,
                            task_manager=task_manager,
                            verbosity=str(args.verbosity),
                            predict_only=False,
                            eval_logger=eval_logger,
                        )
                    except ApplicationError as e:
                        eval_logger.error(
                            "Error generating configurable task.",
                            task=args.task_id,
                            error=str(e),
                            task_name=task_name,
                        )
                        return False, "Error generating configurable task."
                        # raise e
                    except Exception as error:
                        eval_logger.error(
                            "Generate Task raise an error",
                            task_name=task_name,
                            error=error,
                        )
                        eval_logger.error(
                            "Error generating configurable task.",
                            task=args.task_id,
                            error=str(error),
                            task_name=task_name,
                        )
                        return (
                            False,
                            f"Error generating configurable task: {str(error)}",
                        )
                        # raise ApplicationError(
                        #     "Generate TaskDict raise an error",
                        #     str(error),
                        #     type="LmehGenerator",
                        #     non_retryable=True,
                        # )

                    # add another check just in case - does not hurt anybody
                    if not task_dict[task_name]:
                        eval_logger.error(
                            "Missing Task name on TaskDict.",
                            task=args.task_id,
                            task_name=task_name,
                        )
                        return False, "Missing Task name on TaskDict"
                        # raise ApplicationError(
                        #     "Missing Task name on TaskDict",
                        #     task_name,
                        #     type="LmehGenerator",
                        #     non_retryable=False,
                        # )

                    # load dataset from database
                    try:
                        # it is loading data from sql to a dataset
                        await task_dict[task_name].load_from_sql()
                        eval_logger.debug(
                            "Task loaded successfully:", task_dict=task_dict
                        )
                    except ApplicationError as e:
                        eval_logger.error(
                            "Application error loading dataset from SQL.",
                            task=args.task_id,
                            error=str(e),
                            task_name=task_name,
                        )
                        return (
                            False,
                            f"Application error loading dataset from SQL: {str(e)}",
                        )
                        # raise e
                    except Exception as error:
                        error_msg = "Load Dataset from SQL runs in errors"
                        eval_logger.error(
                            error_msg,
                            task=args.task_id,
                            task_name=task_name,
                            error=str(error),
                        )
                        return False, f"{error_msg}: {str(error)}"
                        # raise ApplicationError(
                        #     error_msg, str(error), type="SQLError", non_retryable=True
                        # )

                    try:
                        # Instance LM
                        eval_logger.debug("Generating LM")
                        lm = EvaluatorLM(**args.llm_args)
                        eval_logger.debug("LM generated successfully.")
                        result = await lmeh_generator.evaluate(
                            lm=lm,
                            task_dict=task_dict,
                            task_id=args.task_id,
                            mongo_client=mongo_client,
                            selected_filters=open_llm_filters,
                            selected_metrics=open_llm_metrics,
                            eval_logger=eval_logger,
                        )
                        eval_logger.info(
                            "Evaluation completed successfully.",
                            task_id=str(args.task_id),
                        )
                    except ApplicationError as e:
                        # no mater what, mark the task as drop=True
                        error_msg = "Error during evaluation process"
                        eval_logger.error(
                            error_msg,
                            task=args.task_id,
                            task_name=task_name,
                            error=str(e),
                        )
                        return False, f"{error_msg}: {str(e)}"
                        # raise e
    except Exception as e:
        # TODO: enhance drop task logic
        try:
            await mongo_operator.mark_task_to_drop(args.task_id)
            # Do not rise error, it prevents the manager from being executed, just return
        except Exception as e:
            error_msg = "Failed to mark task to drop."
            eval_logger.error(
                error_msg,
                task=args.task_id,
                task_name=task_name,
                error=str(e),
            )
            return False, f"{error_msg}: {str(e)}"

        # Original error that caused the drop
        return False, f"{error_msg}: {str(e)}"

    return result, "OK"
