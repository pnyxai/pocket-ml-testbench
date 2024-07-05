import logging
import asyncpg
from typing import Optional
from temporalio.exceptions import ApplicationError
from packages.python.lmeh.pocket_lm_eval.tasks import (
    PocketNetworkTaskManager,
    STAGE_TYPING,
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
    task_manager = PocketNetworkTaskManager(
        postgres_conn=postgres_conn,
        verbosity=verbosity,
        include_path=include_path,
        pocket_args=pocket_args,
        stage=stage,
        logger=logger,
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
