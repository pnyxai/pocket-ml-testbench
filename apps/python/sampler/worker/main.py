import asyncio

# import concurrent.futures
import multiprocessing
import sys
from concurrent.futures import ProcessPoolExecutor

from temporalio.client import Client
from temporalio.worker import SharedStateManager, Worker
from temporalio.worker.workflow_sandbox import (
    SandboxedWorkflowRunner,
    SandboxRestrictions,
)

sys.path.append(".")
sys.path.append("../../../")

from activities.lmeh.register_task import register_task as lmeh_register_task
from activities.lmeh.sample import lmeh_sample as lmeh_sample
from activities.signatures.signatures import sign_sample
from app.app import get_app_logger, setup_app
from app.config import read_config
from workflows.register import Register
from workflows.sampler import Sampler

from packages.python.common.utils import get_from_dict

# We always want to pass through external modules to the sandbox that we know
# are safe for workflow use
# this is needed because of https://docs.temporal.io/encyclopedia/python-sdk-sandbox
modules = [
    # internal lib
    "app",
    "activities",
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
    "transformers",
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
    logger.info("starting sampler worker")

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
            Register,
            Sampler,
        ],
        "activities": [
            lmeh_register_task,
            lmeh_sample,
            sign_sample,
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
        eval_logger.info("Interrupted by user. Exiting...")
