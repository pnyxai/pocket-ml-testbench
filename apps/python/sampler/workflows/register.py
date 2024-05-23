from datetime import timedelta
from temporalio import workflow
from temporalio.common import RetryPolicy

# this is needed because of https://docs.temporal.io/encyclopedia/python-sdk-sandbox
with workflow.unsafe.imports_passed_through():
    # add this to ensure app config is available on the thread
    from app.app import get_app_logger, get_app_config
    from protocol.protocol import PocketNetworkRegisterTaskRequest

    # add any activity that needs to be used on this workflow
    from activities.lmeh.register_task import register_task as lmeh_register_task
    from activities.utils import auto_heartbeater

    # lmeh utils
    from activities.lmeh.utils import generator as lmeh_generator
    from activities.lmeh.utils import sql as lmeh_sql

    # lm_eval
    from lm_eval import utils
    from lm_eval.tasks import TaskManager

    # pydantic things
    from pydantic import BaseModel
    from protocol.converter import pydantic_data_converter

    # sql
    import asyncpg

    # datasets
    import datasets

    # async works
    import asyncio


@workflow.defn
class Register:
    @workflow.run
    async def run(self, args: PocketNetworkRegisterTaskRequest) -> bool:
        eval_logger = get_app_logger("Register")
        eval_logger.info("Starting Workflow Register")
        result = False
        if args.framework == "lmeh":
            eval_logger.info("Triggering activity lmeh_register_task")
            result = await workflow.execute_activity(
                lmeh_register_task,
                args,
                start_to_close_timeout=timedelta(seconds=3600),
                #heartbeat_timeout=timedelta(seconds=60),
                retry_policy=RetryPolicy(maximum_attempts=2),
            )
            eval_logger.info("Activity lmeh_register_task done")
        elif args.framework == "helm":
            # TODO: Add helm evaluation
            pass

        eval_logger.info("Workflow Register done")
        return result
