from temporalio import activity
from temporalio.exceptions import ApplicationError

from packages.python.lmeh.utils.common import get_task_manager
from app.app import get_app_logger, get_app_config
from packages.python.protocol.protocol import PocketNetworkTaskRequest
from packages.python.lmeh.utils import generator as lmeh_generator
from packages.python.lmeh.utils import task_config as open_llm_config
from packages.python.lmeh.pocket_lm_eval.models.pocket_network import PocketNetworkLM
from activities.utils import auto_heartbeater
from packages.python.lmeh.utils import sql as lmeh_sql
from packages.python.lmeh.pocket_lm_eval.tasks import TASK_MANAGER_SAMPLE_STAGE


@activity.defn
@auto_heartbeater
async def lmeh_sample(args: PocketNetworkTaskRequest) -> bool:
    app_config = get_app_config()
    eval_logger = get_app_logger("sample")
    config = get_app_config()["config"]
    wf_id = activity.info().workflow_id

    eval_logger.info(
        "Starting activity lmeh_sample",
        task_name=args.tasks,
        requester_args=args.requester_args,
        blacklist=args.blacklist,
        qty=args.qty,
    )
    mongo_client = config["mongo_client"]

    if args.llm_args is None:
        args.llm_args = {}

    eval_logger.debug("Acquiring Postgres Connection from pool")
    async with app_config["postgres"].acquire() as conn:
        async with conn.transaction():
            task_manager, task_names = get_task_manager(
                tasks=args.tasks,
                include_path=args.include_path,
                verbosity=str(args.verbosity),
                logger=eval_logger,
                postgres_conn=conn,
                pocket_args=args,
                stage=TASK_MANAGER_SAMPLE_STAGE,
            )
            eval_logger.debug("Read task names", task_names=task_names)

            for task_name in task_names:
                # lookup the task on task_registry before try to load it
                if not await lmeh_sql.checked_task(task_name, connection=conn):
                    raise ApplicationError(
                        "Task not found on task_registry table",
                        task_name,
                        type="NotFound",
                        non_retryable=True,
                    )

                # generate configurable tasks
                try:
                    open_llm_cfg = open_llm_config.get_task_config(task_names[0])
                    args.num_fewshot = open_llm_cfg["num_fewshot"]
                    task_dict = lmeh_generator.get_configurable_task(
                        tasks=[task_name],
                        num_fewshot=args.num_fewshot,
                        check_integrity=False,
                        gen_kwargs=None,
                        task_manager=task_manager,
                        verbosity=str(args.verbosity),
                        predict_only=False,
                        eval_logger=eval_logger,
                    )
                except ApplicationError as e:
                    raise e
                except Exception as error:
                    eval_logger.error(
                        "Generate Task raise an error", task_name=task_name, error=error
                    )
                    raise ApplicationError(
                        "Generate TaskDict raise an error",
                        str(error),
                        type="LmehGenerator",
                        non_retryable=True,
                    )

                # add another check just in case - does not hurt anybody
                if not task_dict[task_name]:
                    raise ApplicationError(
                        "Missing Task name on TaskDict",
                        task_name,
                        type="LmehGenerator",
                        non_retryable=False,
                    )

                # load dataset from database
                try:
                    # it is loading data from sql to a dataset
                    await task_dict[task_name].load_from_sql()
                    eval_logger.info("Task loaded successfully:", task_dict=task_dict)
                except ApplicationError as e:
                    raise e
                except Exception as error:
                    error_msg = "Load Dataset from SQL runs in errors"
                    eval_logger.error(
                        error_msg,
                        task_name=task_name,
                        error=error,
                    )
                    raise ApplicationError(
                        error_msg, str(error), type="SQLError", non_retryable=True
                    )

                # Instance LM
                eval_logger.info("Generating LM")
                lm = PocketNetworkLM(
                    requester_args=args.requester_args,
                    mongo_client=mongo_client,
                    wf_id=wf_id,
                    **args.llm_args,
                )

                # first load tokenizer then pass it to be used
                await lm.load_tokenizer()

                _ = await lmeh_generator.generate_requests(
                    lm=lm,
                    task_dict=task_dict,
                    mongo_client=mongo_client,
                    args=args,
                    eval_logger=eval_logger,
                )
                eval_logger.info("LM generated successfully.")

    eval_logger.info("Sample Activity done", task_names=task_names)
    return True
