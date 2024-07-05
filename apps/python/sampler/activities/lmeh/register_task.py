from temporalio import activity
from temporalio.exceptions import ApplicationError

from packages.python.lmeh.utils.common import get_task_manager
from app.app import get_app_logger, get_app_config
from packages.python.protocol.protocol import PocketNetworkRegisterTaskRequest
from packages.python.lmeh.pocket_lm_eval.tasks import TASK_MANAGER_REGISTER_STAGE
from packages.python.lmeh.utils import generator as lmeh_generator
from packages.python.lmeh.utils import sql as lmeh_sql
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

    eval_logger.info("Starting activity register task", tasks=args.tasks)

    # retrieve database connection
    eval_logger.debug("Acquiring Postgres Connection from pool")
    async with app_config["postgres"].acquire() as conn:
        async with conn.transaction():
            # if we receive many task names, in theory, all of them could be rollback if something go wrong
            # but if there are too many operations inside will be many intermediate commits, so for the best
            # always sent one task at a time
            task_manager, task_names = get_task_manager(
                tasks=args.tasks,
                include_path=args.include_path,
                verbosity=str(args.verbosity),
                logger=eval_logger,
                postgres_conn=conn,
                stage=TASK_MANAGER_REGISTER_STAGE,
            )
            eval_logger.debug("Read task names", task_names=task_names)
            # sending many task names to the same activity is slower than send a single task to many register workflows
            for task_name in task_names:
                eval_logger.info("Checking Task exists", task_name=task_name)
                # check if the task is already registered
                if not await lmeh_sql.checked_task(task_name, connection=conn):
                    eval_logger.info(
                        "Missing Task. Starting Generation process", task_name=task_name
                    )
                    try:
                        task_dict = lmeh_generator.get_configurable_task(
                            tasks=[task_name],
                            num_fewshot=None,
                            check_integrity=False,
                            gen_kwargs=None,
                            task_manager=task_manager,
                            verbosity=str(args.verbosity),
                            predict_only=False,
                            eval_logger=eval_logger,
                        )
                    except ApplicationError as e:
                        raise e
                    except Exception as error:
                        eval_logger.error(
                            "Generate Task raise an error",
                            task_name=task_name,
                            error=error,
                        )
                        raise ApplicationError(
                            "Generate TaskDict raise an error",
                            str(error),
                            type="LmehGenerator",
                            non_retryable=True,
                        )

                    # add another check just in case - does not hurt anybody
                    if not task_dict[task_name]:
                        raise ApplicationError(
                            "Missing Task name on TaskDict",
                            task_name,
                            type="LmehGenerator",
                            non_retryable=False,
                        )

                    try:
                        # Create dataset table
                        eval_logger.info(
                            "Transferring Task from dataset to postgres",
                            task_name=task_name,
                        )
                        configurable_task = task_dict[task_name]
                        await configurable_task.save_to_sql()
                        # todo: remove line below
                        # await lmeh_sql.create_dataset_table(table_name=table_name, data=data, connection=conn)
                    except ApplicationError as e:
                        raise e
                    except Exception as error:
                        error_msg = "Transfer Dataset to SQL runs in errors"
                        eval_logger.error(
                            error_msg,
                            task_name=task_name,
                            error=error,
                        )
                        raise ApplicationError(
                            error_msg, str(error), type="SQLError", non_retryable=True
                        )

                    try:
                        # Register task/dataset pair
                        await lmeh_sql.register_task(
                            task_name=task_name,
                            dataset_table_name=configurable_task.get_table_name(),
                            connection=conn,
                        )
                    except ApplicationError as e:
                        raise e
                    except Exception as error:
                        error_msg = "Register Task/Dataset pair runs in errors"
                        eval_logger.error(
                            error_msg,
                            task_name=task_name,
                            error=error,
                        )
                        raise ApplicationError(
                            error_msg, str(error), type="SQLError", non_retryable=True
                        )

                    eval_logger.info(
                        "Task registered successfully", task_name=task_name
                    )
                else:
                    eval_logger.info(
                        "ConfigurableTask already registered.", task_name=task_name
                    )

    eval_logger.info("Register Task Activity done")
    return True
