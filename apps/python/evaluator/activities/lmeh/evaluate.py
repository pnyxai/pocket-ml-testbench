from temporalio import activity
from temporalio.exceptions import ApplicationError
from app.app import get_app_logger, get_app_config

import os
import sys

from lm_eval import utils
from lm_eval.tasks import TaskManager
from packages.python.lmeh.pocket_lm_eval.models.pocket_network import EvaluatorLM
# add file path to sys.path
sys.path.append(os.path.dirname(os.path.realpath(__file__)))
# Custom modules
from packages.python.lmeh.utils import generator as lmeh_generator
from packages.python.protocol.protocol import  PocketNetworkEvaluationTaskRequest
from packages.python.lmeh.utils.mongodb import get_doc_ids_by_task, get_task
from packages.python.lmeh.pocket_lm_eval.tasks import PocketNetworkTaskManager
from packages.python.common.auto_heartbeater import auto_heartbeater
from bson import ObjectId


@activity.defn
@auto_heartbeater
async def evaluation(args: PocketNetworkEvaluationTaskRequest) -> bool:
    ############################################################
    # START: POCKET NETWORK CODE
    ############################################################
    app_config = get_app_config()
    eval_logger = get_app_logger("evaluation")
    config = get_app_config()['config']
    wf_id = activity.info().workflow_id
    args.task_id = ObjectId(args.task_id)
    mongo_client = config["mongo_client"]
    if args.llm_args is None:
        args.llm_args = {}
    doc_ids = await get_doc_ids_by_task(args.task_id, mongo_client)
    args.doc_ids = doc_ids
    # Recreate Task request.
    task_mongo = await get_task(args.task_id, mongo_client)
    args.tasks = task_mongo.tasks
    args.blacklist = task_mongo.blacklist
    args.qty = task_mongo.qty
    args.requester_args = task_mongo.requester_args
    args.gen_kwargs = task_mongo.gen_kwargs
    if args.llm_args is None:
        args.llm_args = {}
    args.requester_args = task_mongo.requester_args
    if not task_mongo.done:
        eval_logger.error(f"Task is not done.")
        raise ApplicationError("Task is not done.", task_id= args.task_id, non_retryable=True)
    ############################################################
    # END: POCKET NETWORK CODE
    ############################################################

    eval_logger.debug(f"Starting activity evaluation:", task_id=args.task_id, address=args.requester_args.address,
                        blacklist=args.blacklist, qty=args.qty)
    if args.include_path is not None:
        eval_logger.debug(f"Including path: {args.include_path}")
    task_manager = TaskManager(args.verbosity, include_path=args.include_path)

    if args.tasks is None:
        eval_logger.error("Need to specify task to evaluate.")
        raise ApplicationError("Need to specify task to evaluate.", non_retryable=True)
    elif args.tasks == "list":
        eval_logger.debug(
            "Available Tasks:\n - {}".format("\n - ".join(task_manager.all_tasks))
        )
        raise ApplicationError("Available Tasks:\n - {}".format("\n - ".join(task_manager.all_tasks)),
                                non_retryable=True)
    else:
        if os.path.isdir(args.tasks):
            import glob

            task_names = []
            yaml_path = os.path.join(args.tasks, "*.yaml")
            for yaml_file in glob.glob(yaml_path):
                config = utils.load_yaml_config(yaml_file)
                task_names.append(config)
        else:
            task_list = args.tasks.split(",")
            task_names = task_manager.match_tasks(task_list)
            for task in [task for task in task_list if task not in task_names]:
                if os.path.isfile(task):
                    config = utils.load_yaml_config(task)
                    task_names.append(config)
            task_missing = [
                task for task in task_list if task not in task_names and "*" not in task
            ]  # we don't want errors if a wildcard ("*") task name was used

            if task_missing:
                missing = ", ".join(task_missing)
                eval_logger.error(
                    f"Tasks were not found: {missing}\n"
                    f"{utils.SPACING}Try `lm-eval --tasks list` for list of available tasks",
                )
                raise ApplicationError(
                    f"Tasks not found: {missing}. Try `lm-eval --tasks list` for list of available tasks, or '--verbosity DEBUG' to troubleshoot task registration issues.",
                    non_retryable=True
                )

    eval_logger.info("Generating ConfigurableTask")
    async with app_config["postgres"].acquire() as conn:
        task_manager = PocketNetworkTaskManager(
            postgres_conn=conn,
            verbosity=args.verbosity,
            pocket_args=args,
            logger=eval_logger,
            stage='evaluate'
        )
        try:
            task_dict = lmeh_generator.get_configurable_task(
                tasks=task_names,
                num_fewshot=args.num_fewshot,
                check_integrity=False,
                gen_kwargs=args.gen_kwargs,
                task_manager=task_manager,
                verbosity=str(args.verbosity),
                predict_only=False,
                eval_logger=eval_logger,
            )
            for task_name in task_dict.keys():
                eval_logger.debug(f"Downloading {task_name}")
                eval_logger.debug(f"Task: {task_dict[task_name]}")
                await task_dict[task_name].download()

            eval_logger.info("ConfigurableTask generated successfully:", task_dict=task_dict)
        except ApplicationError as e:
            raise e
        except Exception as error:
            raise ApplicationError(
                "Unexpected error running lmeh_generator.get_configurable_task",
                error,
                type="Unexpected",
                non_retryable=True,
            )

        # Instance LM
        eval_logger.debug("Generating LM")
        lm = EvaluatorLM(**args.llm_args)
        eval_logger.debug("LM generated successfully.")

        results = lmeh_generator.evaluate(
            lm=lm,
            task_dict=task_dict,
            task_id=args.task_id,
            mongo_client=mongo_client,
            eval_logger=eval_logger,
            bootstrap_iters=args.bootstrap_iters,
        )
        eval_logger.info("Evaluation completed successfully.")

    if lm.rank == 0:
        # add info about the model and few shot config
        results["config"] = {
            "model": args.requester_args.address,
            "model_args": args.llm_args,
            "bootstrap_iters": args.bootstrap_iters,
            "gen_kwargs": args.gen_kwargs,
        }
        results["git_hash"] = get_git_commit_hash()
        results["date"] = start_date
        add_env_info(results)  # additional environment info to results

        return results