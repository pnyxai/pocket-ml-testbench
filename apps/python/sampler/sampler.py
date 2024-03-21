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

# Custom modules
from utils.generator import get_ConfigurableTask 

def parse_eval_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(formatter_class=argparse.RawTextHelpFormatter)
    parser.add_argument(
        "--pocket_args",
        type=str,
        default='{"hellaswag": {"address": "random", "__id": [0, 157, 900, 1200]}, "gsmk8": {"address": "random", "__id": [0, 157, 900, 1200]}}',
        help="json string of pocket arguments",
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
    ############################################################
    # START: POCKET NETWORK CODE
    ############################################################
    POSTGRES_DB_USER = args.user
    POSTGRES_DB_PASS = args.password
    POSTGRES_DB_HOST = args.host
    POSTGRES_DB_PORT = args.port
    POSTGRES_DB_NAME = args.dbname

    postgres_uri = "postgresql://{}:{}@{}?port={}&dbname={}".format(
        POSTGRES_DB_USER,
        POSTGRES_DB_PASS,
        POSTGRES_DB_HOST,
        POSTGRES_DB_PORT,
        POSTGRES_DB_NAME,
    )
    # Generate pocket_args from string
    pocket_args = json.loads(args.pocket_args)
    # join keys from pocket_args, adding "," between them
    tasks = ",".join(pocket_args.keys())
    #then add the uri to the pocket_args to be used during ConfigurableTask init
    for k in pocket_args.keys():
        pocket_args[k]['uri'] = postgres_uri
    ############################################################
    # END: POCKET NETWORK CODE
    ############################################################

    task_manager = TaskManager(args.verbosity, include_path=args.include_path)

    if args.include_path is not None:
        eval_logger.info(f"Including path: {args.include_path}")
        include_path(args.include_path)

    if tasks is None:
        eval_logger.error("Need to specify task to evaluate.")
        sys.exit()
    elif tasks == "list":
        eval_logger.info(
            "Available Tasks:\n - {}".format("\n - ".join(task_manager.all_tasks))
        )
        sys.exit()
    else:
        if os.path.isdir(tasks):
            import glob

            task_names = []
            yaml_path = os.path.join(tasks, "*.yaml")
            for yaml_file in glob.glob(yaml_path):
                config = utils.load_yaml_config(yaml_file)
                task_names.append(config)
        else:
            task_list = tasks.split(",")
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
        pocket_args=pocket_args,
    )

    print(task_dict)
    exit()


if __name__ == "__main__":
    cli_register_task()