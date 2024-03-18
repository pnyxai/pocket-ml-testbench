################################
# lm-eval-harness (evaulator.py)
################################
import argparse
import json
import logging
import os
import re
import sys
from functools import partial
from pathlib import Path
from typing import Union

import numpy as np

from lm_eval import evaluator, utils
from lm_eval.evaluator import request_caching_arg_to_dict
from lm_eval.logging_utils import WandbLogger
from lm_eval.tasks import TaskManager, include_path, initialize_tasks
from lm_eval.utils import make_table, simple_parse_args_string

################################
# Custom modules
################################
from utils.uploader import get_ConfigurableTask 
from utils.sql import create_dataset_table, register_task, create_task_table, checked_task
import psycopg2
from psycopg2.extensions import ISOLATION_LEVEL_AUTOCOMMIT

def parse_eval_args() -> argparse.Namespace:
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

if __name__ == "__main__":
    cli_register_task()