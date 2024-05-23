from temporalio import activity
from temporalio.exceptions import ApplicationError
from app.app import get_app_logger, get_app_config

################################
# lm-eval-harness (evaulator.py)
################################
import os
import sys

from lm_eval import utils
from lm_eval.tasks import TaskManager

# add file path to sys.path
sys.path.append(os.path.dirname(os.path.realpath(__file__)))
# Custom modules
from activities.lmeh.utils import generator as lmeh_generator
from activities.lmeh.utils.pocket_lm_eval.models.pocket_network import PocketNetworkLM
from protocol.protocol import PocketNetworkTaskRequest
from activities.lmeh.utils.pocket_lm_eval.tasks import PocketNetworkTaskManager
from activities.utils import auto_heartbeater


@activity.defn
@auto_heartbeater
async def sample(args: PocketNetworkTaskRequest) -> bool:
    app_config = get_app_config()
    eval_logger = get_app_logger("sample")

    wf_id = activity.info().workflow_id
    ############################################################
    # START: POCKET NETWORK CODE
    ############################################################
    config = get_app_config()['config']
    eval_logger.debug(f"Starting activity sample:", task_name=args.tasks, address=args.requester_args.address,
                        blacklist=args.blacklist, qty=args.qty)
    args.postgres_uri = config["postgres_uri"]
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
    # END: POCKET NETWORK CODE
    ############################################################

    eval_logger.debug(f"Verbosity set to {args.verbosity}")
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
        )

        try:
            task_dict = lmeh_generator.get_configurable_task(
                tasks=task_names,
                num_fewshot=None,
                check_integrity=False,
                gen_kwargs=None,
                task_manager=task_manager,
                verbosity=str(args.verbosity),
                predict_only=False,
                eval_logger=eval_logger,
            )
            for task_name in task_dict.keys():
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
        lm = PocketNetworkLM(requester_args=args.requester_args, mongo_client=mongo_client, wf_id=wf_id,
                                **args.llm_args)
        eval_logger.debug("LM generated successfully.")

        _ = lmeh_generator.genererate_requests(
            lm=lm,
            task_dict=task_dict,
            mongo_client=mongo_client,
            args=args,
            eval_logger=eval_logger,
        )

        eval_logger.info("Request generated successfully:", task_names=task_names)

        return True