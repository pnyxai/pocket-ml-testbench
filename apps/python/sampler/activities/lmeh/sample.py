from temporalio import activity
from temporalio.exceptions import ApplicationError
from app.app import get_app_logger, get_app_config

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
from lm_eval.tasks import TaskManager
from lm_eval.utils import make_table, simple_parse_args_string

# add file path to sys.path
sys.path.append(os.path.dirname(os.path.realpath(__file__)))
# Custom modules
from activities.lmeh.utils import generator as lmeh_generator
from activities.lmeh.utils import mongodb as lmeh_mongodb 
from activities.lmeh.utils.pocket_lm_eval.models.pocket_network import PocketNetworkLM
from protocol.protocol import PocketNetworkTaskRequest, PocketNetworkMongoDBTask
from activities.lmeh.utils.pocket_lm_eval.tasks import PocketNetworkTaskManager

from pymongo import MongoClient
from bson.objectid import ObjectId
from collections import defaultdict
from dataclasses import asdict


@activity.defn
async def sample(args: PocketNetworkTaskRequest) -> bool:
    ############################################################
    # START: POCKET NETWORK CODE
    ############################################################
    config = get_app_config()['config']
    eval_logger = get_app_logger("sample")
    eval_logger.info(f"Starting activity sample:", task_name=args.tasks, address=args.requester_args.address, blacklist=args.blacklist, qty=args.qty)
    args.postgres_uri = config["postgres_uri"]
    args.mongodb_uri = config["mongodb_uri"]
    mongo_client = config["mongo_client"]
    try:
        # The ping command is cheap and does not require auth.
        mongo_client.admin.command('ping')
    except Exception as e:
        eval_logger.error(f"Mongo DB connection failed.")
        raise ApplicationError("Mongo DB connection failed.", non_retryable=True)
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

    eval_logger.info("Generating ConfigurableTask")
    task_manager = PocketNetworkTaskManager(args.verbosity, pocket_args=args, logger=eval_logger)
    try:
        task_dict = lmeh_generator.get_configurable_task(
            tasks = task_names,
            num_fewshot = None,
            check_integrity = False,
            gen_kwargs = None,
            task_manager = task_manager,
            verbosity =  args.verbosity,
            predict_only = False,
            eval_logger = eval_logger,
        )
        eval_logger.info("ConfigurableTask generated successfully:", task_dict=task_dict)
    except ApplicationError as e:
        raise e
    except Exception as e:
        raise e
    
    # Instance LM
    eval_logger.info("Generating LM")
    lm = PocketNetworkLM(requester_args = args.requester_args, mongo_client = mongo_client, **args.llm_args)
    eval_logger.info("LM generated successfully.")

    requests = lmeh_generator.genererate_requests(lm=lm,
                                            task_dict=task_dict,
                                            mongo_client=mongo_client,
                                            args=args,
                                            eval_logger=eval_logger,
                                            )

    eval_logger.info("Request generated successfully:", task_names=task_names)

    return True
