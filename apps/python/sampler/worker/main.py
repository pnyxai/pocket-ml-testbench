import asyncio
import sys
import concurrent.futures
from concurrent.futures import ProcessPoolExecutor
import multiprocessing

from temporalio.client import Client
from temporalio.worker import SharedStateManager, Worker
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

# We always want to pass through external modules to the sandbox that we know
# are safe for workflow use
with workflow.unsafe.imports_passed_through():
    from pydantic import BaseModel

    from protocol.converter import pydantic_data_converter

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
    # temporal_host = "localhost:7233"
    namespace = get_from_dict(config, 'temporal.namespace')
    # namespace = "pocket-ml-testbench"
    task_queue = get_from_dict(config, 'temporal.task_queue')
    # task_queue = "sampler-local"
    max_workers = get_from_dict(config, "temporal.max_workers")

    client = await Client.connect(
        temporal_host,
        namespace=namespace,
        #data_converter=pydantic_data_converter
    )

    worker = Worker(
        client,
        task_queue=task_queue,
        workflows=[Register, Sampler],
        activities=[lmeh_register_task, lmeh_sample],
        # Synchronous activities are not allowed unless we provide some kind of
        # executor. Here we are giving a process pool executor which means the
        # activity will actually run in a separate process. This same executor
        # could be passed to multiple workers if desired.
        activity_executor=ProcessPoolExecutor(max_workers),
        # Since we are using an executor that is not a thread pool executor,
        # Temporal needs some kind of manager to share state such as
        # cancellation info and heartbeat info between the host and the
        # activity. Therefore, we must provide a shared_state_manager here. A
        # helper is provided to create it from a multiprocessing manager.
        shared_state_manager=SharedStateManager.create_from_multiprocessing(
            multiprocessing.Manager()
        )
    )

    await worker.run()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("interrupted by user. Exiting...")