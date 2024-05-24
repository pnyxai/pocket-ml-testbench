from temporalio import activity
from temporalio.exceptions import ApplicationError
from lm_eval.tasks import TaskManager

from app.app import get_app_logger, get_app_config
from protocol.protocol import PocketNetworkRegisterTaskRequest
from activities.lmeh.utils import generator as lmeh_generator
from activities.lmeh.utils import sql as lmeh_sql
from activities.utils import auto_heartbeater


@activity.defn
@auto_heartbeater
async def register_task(args: PocketNetworkRegisterTaskRequest) -> bool:
    """
    LM Evaluation Harness dataset uploading.

    This function takes the selected tasks and fills the database with all required datasets.
    """
    eval_logger = get_app_logger("register_task")
    app_config = get_app_config()

    eval_logger.info("Starting activity register task", task=args.tasks)

    ############################################################
    # START: LM-EVAL-HARNESS CODE
    ############################################################
    if args.include_path is not None:
        eval_logger.debug(f"Including path: {args.include_path}", include_path=args.include_path)

    task_manager = TaskManager(args.verbosity, include_path=args.include_path)

    if args.tasks is None:
        eval_logger.error("Need to specify task to evaluate.")
        raise ApplicationError("Need to specify task to evaluate.", args, type="BadParams", non_retryable=True)
    else:
        task_list = args.tasks.split(",")
        task_names = task_manager.match_tasks(task_list)

        task_missing = [
            task for task in task_list if task not in task_names and "*" not in task
        ]  # we don't want errors if a wildcard ("*") task name was used

        if task_missing:
            missing_tasks = ", ".join(task_missing)
            eval_logger.error("Tasks were not found", missing_tasks=missing_tasks)
            raise ApplicationError("Tasks not found", missing_tasks, type="TaskNotFound", non_retryable=True)

    ############################################################
    # END: LM-EVAL-HARNESS CODE
    ############################################################

    eval_logger.debug("task_names", task_names=task_names)

    # retrieve database connection
    async with app_config["postgres"].acquire() as conn:

        async with conn.transaction():
            for task_name in task_names:
                eval_logger.info("Checking ConfigurableTask exists", task_name=task_name)
                # check if the task is already registered
                if not await lmeh_sql.checked_task(task_name, connection=conn):
                    try:
                        eval_logger.warn("Missing ConfigurableTask", task_name=task_name)
                        eval_logger.info("Generating ConfigurableTask", task_name=task_name)
                        task_dict = lmeh_generator.get_configurable_task(
                            tasks=[task_name],
                            num_fewshot=None,
                            check_integrity=False,
                            gen_kwargs=None,
                            task_manager=task_manager,
                            verbosity=str(args.verbosity),
                            predict_only=False,
                            eval_logger=eval_logger
                        )
                        eval_logger.info("ConfigurableTask generation successful", task_name=task_name)
                    except Exception as error:
                        eval_logger.error("Generating ConfigurableTask run in errors", task_name=task_name, error=error)
                        raise ApplicationError(
                            "Generating ConfigurableTask run in errors",
                            str(error),
                            type="LmehGenerator",
                            non_retryable=True,
                        )

                    dataset_path = task_dict[task_name].config.dataset_path
                    dataset_name = task_dict[task_name].config.dataset_name
                    table_name = dataset_path + "--" + dataset_name if dataset_name else dataset_path
                    data = task_dict[task_name].dataset

                    # Register task
                    try:
                        # Create dataset table
                        await lmeh_sql.create_dataset_table(table_name=table_name, data=data, connection=conn)
                        # Register task/dataset pair
                        await lmeh_sql.register_task(
                            task_name=task_name,
                            dataset_table_name=table_name,
                            connection=conn,
                        )
                    except Exception as error:
                        error_msg = "SQL Statements for ConfigurableTask runs in errors"
                        eval_logger.error(error_msg, task_name=task_name, error=error, )
                        raise ApplicationError(
                            error_msg,
                            str(error),
                            type="SQLError",
                            non_retryable=True
                        )

                    eval_logger.info("ConfigurableTask registered.", task_name=task_name)
                else:
                    eval_logger.info("ConfigurableTask already registered.", task_name=task_name)

    eval_logger.info("Register Task Activity done")
    return True
