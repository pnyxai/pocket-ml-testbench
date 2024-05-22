import asyncio
import sys

from temporalio.client import Client
from temporalio.worker import Worker, SharedStateManager
from temporalio import workflow

sys.path.append('.')
sys.path.append('../../../')

from packages.python.common.utils import get_from_dict
from app.app import setup_app, get_app_logger
from app.config import read_config

from activities.lmeh.register_task import register_task as lmeh_register_task
from activities.lmeh.sample import sample as lmeh_sample
from workflows.register import Register
from workflows.sampler import Sampler
import multiprocessing
from concurrent.futures import ProcessPoolExecutor
from concurrent.futures import ThreadPoolExecutor
import concurrent.futures

# We always want to pass through external modules to the sandbox that we know
# are safe for workflow use
with workflow.unsafe.imports_passed_through():
    from pydantic import BaseModel
    from protocol.converter import pydantic_data_converter

interrupt_event = asyncio.Event()


async def main():
    """
    Main method for running the worker.

    :return: None
    """
    cfg = read_config()

    app_config = setup_app(cfg)

    config = app_config["config"]

    l = get_app_logger("worker")
    l.info("starting worker")

    temporal_host = f"{get_from_dict(config, 'temporal.host')}:{get_from_dict(config, 'temporal.port')}"
    namespace = get_from_dict(config, 'temporal.namespace')
    task_queue = get_from_dict(config, 'temporal.task_queue')
    max_workers = get_from_dict(config, "temporal.max_workers")

    client = await Client.connect(
        temporal_host,
        namespace=namespace,
        # data_converter=pydantic_data_converter
    )

    with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as activity_executor:
        worker = Worker(
            client,
            task_queue=task_queue,
            workflows=[
                Register,
                Sampler,
            ],
            activities=[
                lmeh_register_task,
                lmeh_sample,
            ],
            activity_executor=activity_executor,
            max_concurrent_activities=max_workers,
            max_concurrent_workflow_tasks=max_workers,
            max_concurrent_workflow_task_polls=max_workers,
            max_concurrent_activity_task_polls=max_workers
        )

        await worker.run()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        eval_logger = get_app_logger("Main")
        eval_logger.info("interrupted by user. Exiting...")
