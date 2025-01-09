import logging
import random
from typing import Optional

import asyncpg
import numpy as np
import torch
from temporalio.exceptions import ApplicationError

from packages.python.lmeh.pocket_lm_eval.tasks import (
    STAGE_TYPING,
    PocketNetworkTaskManager,
)
from packages.python.protocol.protocol import PocketNetworkTaskRequest


def get_task_manager(
    tasks: str,
    include_path: str,
    verbosity: str,
    postgres_conn: asyncpg.Connection,
    logger: Optional[logging.Logger] = None,
    pocket_args: Optional[PocketNetworkTaskRequest] = None,
    stage: Optional[STAGE_TYPING] = None,
    random_seed: Optional[int] = None,
    numpy_random_seed: Optional[int] = None,
    torch_random_seed: Optional[int] = None,
    hf_token: Optional[str] = None,
):
    """
    :param stage:
    :param pocket_args:
    :param postgres_conn:
    :param tasks: A string representing the tasks to be evaluated. Each task should be separated by a comma.
    :param include_path: A string representing the path to include when searching for tasks.
    :param verbosity: A string representing the verbosity level for the task manager.
    :param logger: An object representing the logger to be used for logging messages.

    :return: A tuple containing the task manager object and a list of matched task names.

    """

    seed_message = []
    if random_seed is not None:
        # See https://github.com/EleutherAI/lm-evaluation-harness/pull/1412
        seed_message.append(f"Setting random seed to {random_seed}")
        random.seed(random_seed)
    else:
        random_seed = random.randint(0, 2**32 - 1)
        random.seed(random_seed)

    if numpy_random_seed is not None:
        seed_message.append(f"Setting numpy seed to {numpy_random_seed}")
        np.random.seed(numpy_random_seed)
    else:
        numpy_random_seed = random.randint(0, 2**32 - 1)
        np.random.seed(numpy_random_seed)

    if torch_random_seed is not None:
        seed_message.append(f"Setting torch manual seed to {torch_random_seed}")
        torch.manual_seed(torch_random_seed)
    else:
        torch_random_seed = torch.randint(0, 2**32 - 1, (1,)).item()
        torch.manual_seed(torch_random_seed)

    if seed_message:
        logger.info(" | ".join(seed_message))

    task_manager = PocketNetworkTaskManager(
        postgres_conn=postgres_conn,
        verbosity=verbosity,
        include_path=include_path,
        pocket_args=pocket_args,
        stage=stage,
        logger=logger,
        hf_token=hf_token,
    )

    if tasks is None:
        logger.error("Need to specify task to evaluate.")
        raise ApplicationError(
            "Need to specify task to evaluate.",
            tasks,
            include_path,
            verbosity,
            type="BadParams",
            non_retryable=True,
        )
    else:
        task_list = tasks.split(",")
        task_names = task_manager.match_tasks(task_list)

        task_missing = [
            task for task in task_list if task not in task_names and "*" not in task
        ]  # we don't want errors if a wildcard ("*") task name was used

        if task_missing:
            missing_tasks = ", ".join(task_missing)
            # noinspection PyArgumentList
            logger.error("Tasks were not found", missing_tasks=missing_tasks)
            raise ApplicationError(
                "Tasks not found",
                missing_tasks,
                type="TaskNotFound",
                non_retryable=True,
            )

    return task_manager, task_names
