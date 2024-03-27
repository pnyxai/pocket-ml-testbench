import argparse
import logging
import os
import sys
from typing import Union
from lm_eval import  utils
from lm_eval.tasks import TaskManager, include_path, initialize_tasks
from lm_eval.utils import eval_logger, positional_deprecated, simple_parse_args_string
from lm_eval.tasks import TaskManager, get_task_dict
from lm_eval.evaluator_utils import run_task_tests

from typing import List, Optional, Union


from utils.sql import create_dataset_table, register_task, create_task_table, checked_task
import psycopg2




def parse_eval_args() -> argparse.Namespace:
    '''
    Argument parsing for LM-Evaluation-Harness dataset uploading.
    '''
    parser = argparse.ArgumentParser(formatter_class=argparse.RawTextHelpFormatter)
    parser.add_argument(
        "--tasks",
        "-t",
        default=None,
        metavar="task1,task2",
        help="To get full list of tasks, use the command lm-eval --tasks list",
    )
    parser.add_argument(
        "--dbname",
        type=str,
        default="postgres",
        help="Name of the database",
    )
    parser.add_argument(
        "--user",
        type=str,
        default="postgres",
        help="Name of the user",
    )
    parser.add_argument(
        "--password",
        type=str,
        default="password",
        help="Password for the user",
    )
    parser.add_argument(
        "--host",
        type=str,
        default="localhost",
        help="Host name",
    )
    parser.add_argument(
        "--port",
        type=str,
        default="5432",
        help="Port number",
    )
    parser.add_argument(
        "--include_path",
        type=str,
        default=None,
        metavar="DIR",
        help="Additional path to include if there are external tasks to include.",
    )    
    parser.add_argument(
        "--verbosity",
        "-v",
        type=str.upper,
        default="INFO",
        metavar="CRITICAL|ERROR|WARNING|INFO|DEBUG",
        help="Controls the reported logging error level. Set to DEBUG when testing + adding new task configurations for comprehensive log output.",
    )        
    return parser.parse_args()

def cli_register_task(args: Union[argparse.Namespace, None] = None) -> None:
    '''
    LM Evaluation Harness dataset uploading.

    This function takes the selected tasks and fill the database with all 
    requiered datasets.
    '''
    if not args:
        # we allow for args to be passed externally, else we parse them ourselves
        args = parse_eval_args()

    eval_logger = utils.eval_logger
    eval_logger.setLevel(getattr(logging, f"{args.verbosity}"))
    eval_logger.info(f"Verbosity set to {args.verbosity}")

    initialize_tasks(args.verbosity)
    task_manager = TaskManager(args.verbosity, include_path=args.include_path)

    if args.include_path is not None:
        eval_logger.info(f"Including path: {args.include_path}")
        include_path(args.include_path)

    if args.tasks is None:
        eval_logger.error("Need to specify task to evaluate.")
        sys.exit()
    elif args.tasks == "list":
        eval_logger.info(
            "Available Tasks:\n - {}".format("\n - ".join(task_manager.all_tasks))
        )
        sys.exit()
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
                raise ValueError(
                    f"Tasks not found: {missing}. Try `lm-eval --tasks list` for list of available tasks, or '--verbosity DEBUG' to troubleshoot task registration issues."
                )

    task_dict = get_ConfigurableTask(
        tasks=task_names,
        num_fewshot=None,
        check_integrity=False,
        gen_kwargs=None,
        task_manager= None,
        verbosity= "INFO",
        predict_only= False,    
    )

    # check and connect to the database
    try:
        conn = psycopg2.connect(
            dbname=args.dbname,
            user=args.user,
            password=args.password,
            host=args.host,
            port=args.port
        )
        eval_logger.info("Connected to the database")
        # Obtain a DB Cursor
        cursor = conn.cursor()
    except Exception as e:
        eval_logger.error("Unable to connect to the database")
        exit(-1)

    create_task_table(connection=conn)

    for t in task_dict:
        task_name_i = t
        dataset_path = task_dict[t].config.dataset_path
        dataset_name = task_dict[t].config.dataset_name
        table_name = dataset_path + "--" + dataset_name if dataset_name else dataset_path
        data = task_dict[t].dataset
        # check if the task is already registered
        if not checked_task(task_name_i, connection= conn):
            # Register task
            try:
                # Create dataset table
                create_dataset_table(table_name = table_name, 
                                    data = data, 
                                    connection = conn)
                # Regist task/dataset pair
                register_task(task_name = task_name_i, 
                            dataset_table_name = table_name,
                            connection = conn)
            except Exception as e:
                eval_logger.error(f"Error: {e}")
                conn.rollback()
                cursor.close()
                conn.close()
                exit(-1)
            eval_logger.info(f"Task {task_name_i} registered successfully")
        else:
            eval_logger.info(f"Task {task_name_i} already registered")


# cutted def simple_evaluate(..) from lm-eval-harness to generate config task commit:7d9922c80114218eaf43975b7655bb48cda84f50
@positional_deprecated
def get_ConfigurableTask(
    tasks: Optional[List[Union[str, dict, object]]] = None,
    num_fewshot: Optional[int] = None,
    check_integrity: bool = False,
    gen_kwargs: Optional[str] = None,
    task_manager: Optional[TaskManager] = None,
    verbosity: str = "INFO",
    predict_only: bool = False,

):
    """Instantiate and evaluate a model on a list of tasks.

    :param tasks: list[Union[str, dict, Task]]
        List of task names or Task objects. Task objects will be taken to have name task.EVAL_HARNESS_NAME if defined and type(task).__name__ otherwise.
    :param num_fewshot: int
        Number of examples in few-shot context
    :param check_integrity: bool
        Whether to run the relevant part of the test suite for the tasks
    :param gen_kwargs: str
        String arguments for model generation
        Ignored for all tasks with loglikelihood output_type
    :param predict_only: bool
        If true only model outputs will be generated and returned. Metrics will not be evaluated

    :return
        Task dictionary
    """
    eval_logger.setLevel(getattr(logging, f"{verbosity}"))

    seed_message = []

    if seed_message:
        eval_logger.info(" | ".join(seed_message))

    if tasks is None:
        tasks = []
    if len(tasks) == 0:
        raise ValueError(
            "No tasks specified, or no tasks found. Please verify the task names."
        )

    if gen_kwargs is not None:
        gen_kwargs = simple_parse_args_string(gen_kwargs)
        eval_logger.warning(
            "generation_kwargs specified through cli, these settings will update set parameters in yaml tasks. "
            "Ensure 'do_sample=True' for non-greedy decoding!"
        )
        if gen_kwargs == "":
            gen_kwargs = None

    if task_manager is None:
        task_manager = TaskManager(verbosity)

    eval_logger.info(
        "get_task_dict has been updated to accept an optional argument, `task_manager`"
        "Read more here:https://github.com/EleutherAI/lm-evaluation-harness/blob/main/docs/interface.md#external-library-usage"
    )
    task_dict = get_task_dict(tasks, task_manager)
    for task_name in task_dict.keys():
        task_obj = task_dict[task_name]
        if isinstance(task_obj, tuple):
            _, task_obj = task_obj
            if task_obj is None:
                continue

        if task_obj.get_config("output_type") == "generate_until":
            if gen_kwargs is not None:
                task_obj.set_config(
                    key="generation_kwargs", value=gen_kwargs, update=True
                )

        if predict_only:
            log_samples = True
            eval_logger.info(
                f"Processing {task_name} in output-only mode. Metrics will not be calculated!"
            )
            # we have to change the class properties post-hoc. This is pretty hacky.
            task_obj.override_metric(metric_name="bypass")

        if num_fewshot is not None:
            if (default_num_fewshot := task_obj.get_config("num_fewshot")) == 0:
                eval_logger.info(
                    f"num_fewshot has been set to 0 for {task_name} in its config. Manual configuration will be ignored."
                )
            else:
                eval_logger.warning(
                    f"Overwriting default num_fewshot of {task_name} from {default_num_fewshot} to {num_fewshot}"
                )
                task_obj.set_config(key="num_fewshot", value=num_fewshot)

    if check_integrity:
        run_task_tests(task_list=tasks)

    return task_dict
