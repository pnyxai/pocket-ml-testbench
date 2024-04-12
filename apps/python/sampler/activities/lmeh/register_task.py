from temporalio import activity
from temporalio.exceptions import ApplicationError
from app.app import get_app_logger, get_app_config
from protocol.protocol import PocketNetworkRegisterTaskRequest

## LM Evaluation Harness
import os
import sys

from lm_eval import  utils
from lm_eval.tasks import TaskManager

# add file path to sys.path
sys.path.append(os.path.dirname(os.path.realpath(__file__)))
from activities.lmeh.utils import register as lmeh_register
from activities.lmeh.utils import sql as lmeh_sql
from urllib.parse import urlparse
import psycopg2



@activity.defn
async def register_task(args: PocketNetworkRegisterTaskRequest) -> bool:
    '''
    LM Evaluation Harness dataset uploading.

    This function takes the selected tasks and fill the database with all 
    requiered datasets.
    '''
    config = get_app_config()['config']
    eval_logger = get_app_logger("register_task")
    eval_logger.info(f"Starting activity register task:", task=args.tasks)   

    ############################################################
    # START: LM-EVAL-HARNESS CODE
    ############################################################
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
        raise ApplicationError("Available Tasks:\n - {}".format("\n - ".join(task_manager.all_tasks)), non_retryable=True)
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
    ############################################################
    # END: LM-EVAL-HARNESS CODE
    ############################################################
    eval_logger.debug(f"task_names",task_names=task_names, type=type(task_names))

    # check and connect to the database
    try:
        # Parse the URI to extract connection parameters
        uri_parts = urlparse(config["postgres_uri"])
        dbname = uri_parts.path[1:]
        username = uri_parts.username
        password = uri_parts.password
        host = uri_parts.hostname
        port = uri_parts.port        
        conn = psycopg2.connect(
            dbname=dbname,
            user=username,
            password=password,
            host=host,
            port=port
        )
        eval_logger.debug("Connected to the database")
        # Obtain a DB Cursor
        cursor = conn.cursor()
    except Exception as e:
        eval_logger.error("Unable to connect to the database")
        raise ApplicationError("Unable to connect to the database", non_retryable=True)

    # Create the task table if it does not exist
    lmeh_sql.create_task_table(connection=conn)

    for task_name_i in task_names:
        # check if the task is already registered
        if not lmeh_sql.checked_task(task_name_i, connection= conn):
            try:
                eval_logger.info("Generating ConfigurableTask", task=task_name_i)
                task_dict_i = lmeh_register.get_ConfigurableTask(
                    tasks=[task_name_i],
                    num_fewshot=None,
                    check_integrity=False,
                    gen_kwargs=None,
                    task_manager= None,
                    verbosity= "INFO",
                    predict_only= False,    
                )
                eval_logger.info("ConfigurableTask generatation successful")
            except Exception as e:
                eval_logger.error(f"Error: {e}",task_name=task_name_i)
                cursor.close()
                conn.close()
                raise ApplicationError(f"Error: {e}", non_retryable=True)
            dataset_path = task_dict_i[task_name_i].config.dataset_path
            dataset_name = task_dict_i[task_name_i].config.dataset_name
            table_name = dataset_path + "--" + dataset_name if dataset_name else dataset_path
            data = task_dict_i[task_name_i].dataset
            # Register task
            try:
                # Create dataset table
                lmeh_sql.create_dataset_table(table_name = table_name, 
                                    data = data, 
                                    connection = conn)
                # Regist task/dataset pair
                lmeh_sql.register_task(task_name = task_name_i, 
                            dataset_table_name = table_name,
                            connection = conn)
            except Exception as e:
                eval_logger.error(f"Error: {e}",task_name=task_name_i)
                conn.rollback()
                cursor.close()
                conn.close()
                raise ApplicationError(f"Error: {e}", non_retryable=True)
            eval_logger.debug("Task registered:" ,task_name=task_name_i)
        else:
            eval_logger.debug("Task already registered:",task_name=task_name_i)

    # Commit the transaction and close the cursor
    conn.commit()
    cursor.close()

    return True