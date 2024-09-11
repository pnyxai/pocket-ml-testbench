import asyncio
import sys
import multiprocessing
from concurrent.futures import ProcessPoolExecutor
from temporalio.client import Client
from temporalio.worker import Worker, SharedStateManager
from temporalio.worker.workflow_sandbox import (
    SandboxedWorkflowRunner,
    SandboxRestrictions,
)

sys.path.append(".")
sys.path.append("../../../")

from packages.python.common.utils import get_from_dict
from app.app import setup_app, get_app_logger
from app.config import read_config

from activities.lmeh.evaluate import lmeh_evaluate
from activities.get_task_data import get_task_data
from activities.lookup_tasks import lookup_tasks
from activities.signatures.tokenizer_evaluate import tokenizer_evaluate
from activities.signatures.model_config_evaluate import model_config_evaluate

from workflows.evaluator import Evaluator
from workflows.lookup_tasks import LookupTasks

# We always want to pass through external modules to the sandbox that we know
# are safe for workflow use
# this is needed because of https://docs.temporal.io/encyclopedia/python-sdk-sandbox
modules = [
    # internal lib
    "app",
    "activities",
    "protocol",
    "packages.python.protocol",
    "packages.python.common",
    "packages.python.logger",
    "packages.python.lmeh",
    # external lib
    "motor",
    "asyncpg",
    "asyncio",
    "lm_eval",
    "pydantic",
    "datasets",
]


async def main():
    """
    Main method for running the worker.

    :return: None
    """
    cfg = read_config()

    app_config = await setup_app(cfg)

    config = app_config["config"]

    logger = get_app_logger("worker")
    logger.info("starting worker")

    temporal_host = f"{get_from_dict(config, 'temporal.host')}:{get_from_dict(config, 'temporal.port')}"
    namespace = get_from_dict(config, "temporal.namespace")
    task_queue = get_from_dict(config, "temporal.task_queue")
    max_workers = get_from_dict(config, "temporal.max_workers")
    max_concurrent_activities = get_from_dict(
        config, "temporal.max_concurrent_activities"
    )
    max_concurrent_workflow_tasks = get_from_dict(
        config, "temporal.max_concurrent_workflow_tasks"
    )
    max_concurrent_workflow_task_polls = get_from_dict(
        config, "temporal.max_concurrent_workflow_task_polls"
    )
    max_concurrent_activity_task_polls = get_from_dict(
        config, "temporal.max_concurrent_activity_task_polls"
    )

    client = await Client.connect(
        temporal_host,
        namespace=namespace,
        # data_converter=pydantic_data_converter
    )
    app_config["temporal_client"] = client

    worker_kwargs = {
        "client": client,
        "task_queue": task_queue,
        "activity_executor": ProcessPoolExecutor(max_workers),
        "shared_state_manager": SharedStateManager.create_from_multiprocessing(
            multiprocessing.Manager()
        ),
        "workflow_runner": SandboxedWorkflowRunner(
            restrictions=SandboxRestrictions.default.with_passthrough_modules(*modules)
        ),
        "workflows": [
            Evaluator,
            LookupTasks,
        ],
        "activities": [
            lookup_tasks,
            get_task_data,
            lmeh_evaluate,
            tokenizer_evaluate,
            model_config_evaluate,
        ],
    }

    if max_concurrent_activities is not None:
        worker_kwargs["max_concurrent_activities"] = max_concurrent_activities
    if max_concurrent_workflow_tasks is not None:
        worker_kwargs["max_concurrent_workflow_tasks"] = max_concurrent_workflow_tasks
    if max_concurrent_workflow_task_polls is not None:
        worker_kwargs["max_concurrent_workflow_task_polls"] = (
            max_concurrent_workflow_task_polls
        )
    if max_concurrent_activity_task_polls is not None:
        worker_kwargs["max_concurrent_activity_task_polls"] = (
            max_concurrent_activity_task_polls
        )

    worker = Worker(**worker_kwargs)

    await worker.run()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        eval_logger = get_app_logger("Main")
        eval_logger.info("interrupted by user. Exiting...")
