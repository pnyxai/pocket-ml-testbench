from activities.utils import auto_heartbeater
from app.app import get_app_config, get_app_logger
from temporalio import activity
from temporalio.exceptions import ApplicationError

from lm_eval.utils import simple_parse_args_string
from packages.python.lmeh.pocket_lm_eval.models.pocket_network import (
    SamplerCompletionAPI,
    SamplerChatCompletionAPI,
)
from packages.python.lmeh.pocket_lm_eval.tasks import TASK_MANAGER_SAMPLE_STAGE
from packages.python.lmeh.utils import generator as lmeh_generator
from packages.python.lmeh.utils import sql as lmeh_sql
from packages.python.lmeh.utils import task_config as open_llm_config
from packages.python.lmeh.utils.common import get_task_manager
from packages.python.protocol.protocol import (
    LLMTimeouts,
    PocketNetworkTaskRequest,
    TimeoutHandler,
)


@activity.defn
@auto_heartbeater
async def lmeh_sample(args: PocketNetworkTaskRequest) -> bool:
    app_config = get_app_config()
    eval_logger = get_app_logger("sample")
    config = get_app_config()["config"]
    wf_id = activity.info().workflow_id
    # check if config has timeouts
    if "timeouts" in config:
        try:
            # Try to get this service timeouts
            timeout_cfg = config["timeouts"].get(args.requester_args.service, None)
            # Check if not there
            if timeout_cfg is None:
                # Check for default
                timeout_cfg = config["timeouts"].get("default", None)
                if timeout_cfg is None:
                    # Initialize as empy
                    timeout_handler = TimeoutHandler()
                    eval_logger.warn(
                        "TimeoutHandler config not found and no default timeout is defined, using EMPTY TIMEOUT",
                        service=args.requester_args.service,
                    )
                else:
                    # Initialize as default
                    eval_logger.info(
                        "TimeoutHandler config not found, using DEFAULT TIMEOUT",
                        service=args.requester_args.service,
                    )
                    timeouts = LLMTimeouts(**timeout_cfg)
                    timeout_handler = TimeoutHandler(timeouts=timeouts)
            else:
                timeouts = LLMTimeouts(**timeout_cfg)
                timeout_handler = TimeoutHandler(timeouts=timeouts)
        except Exception as e:
            eval_logger.error(
                "Error creating TimeoutHandler",
                error=e,
                timeouts=config["timeouts"],
                service=args.requester_args.service,
            )
            raise ApplicationError(
                "Error creating TimeoutHandler",
                str(e),
                type="TimeoutHandler",
                non_retryable=True,
            )
    else:
        timeout_handler = TimeoutHandler()

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

    # Check include path and override with config
    # TODO : This should not be an argument from the request
    include_path = args.include_path
    if "include_path" in config:
        include_path = config["include_path"]
        eval_logger.info(
            f"Using additional tasks from : {include_path}",
        )

    metadata = (
        simple_parse_args_string(args.llm_args)
        if isinstance(args.llm_args, str)
        else args.llm_args
        if isinstance(args.llm_args, dict)
        else {}
        # ) | (
        #     args.metadata
        #     if isinstance(args.metadata, dict)
        #     else simple_parse_args_string(args.metadata)
    )

    eval_logger.debug("Acquiring Postgres Connection from pool")
    async with app_config["postgres"].acquire() as conn:
        async with conn.transaction():
            task_manager, task_names = get_task_manager(
                tasks=args.tasks,
                include_path=include_path,
                verbosity=str(args.verbosity),
                postgres_conn=conn,
                logger=eval_logger,
                metadata=metadata,
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

                # Generate configurable tasks
                try:
                    # Loading task config and updating args when needed
                    open_llm_cfg = open_llm_config.get_task_config(task_name)
                    if "num_fewshot" in open_llm_cfg:
                        args.num_fewshot = open_llm_cfg["num_fewshot"]
                    if "system_instruction" in open_llm_cfg:
                        args.system_instruction = open_llm_cfg["system_instruction"]
                    if "apply_chat_template" in open_llm_cfg:
                        args.apply_chat_template = open_llm_cfg["apply_chat_template"]
                    if "fewshot_as_multiturn" in open_llm_cfg:
                        args.fewshot_as_multiturn = open_llm_cfg["fewshot_as_multiturn"]
                    if "gen_kwargs" in open_llm_cfg:
                        args.gen_kwargs = open_llm_cfg["gen_kwargs"]
                    if "path" in open_llm_cfg:
                        args.requester_args.path = open_llm_cfg["path"]
                    # Validate if fewshot_as_multiturn and apply_chat_template are set correctly
                    if args.fewshot_as_multiturn and args.apply_chat_template is False:
                        eval_logger.error(
                            "When `fewshot_as_multiturn` is selected, `apply_chat_template` must be set (either to `True` or to the chosen template name).",
                            task_name=task_name,
                            fewshot_as_multiturn=args.fewshot_as_multiturn,
                            apply_chat_template=args.apply_chat_template,
                        )
                        raise ApplicationError(
                            "When `fewshot_as_multiturn` is selected, `apply_chat_template` must be set (either to `True` or to the chosen template name).",
                            type="BadParams",
                            non_retryable=True,
                        )
                    # Now all the args are set, we can generate the task dict with ConfigurableTask
                    task_dict = lmeh_generator.get_configurable_task(
                        tasks=[task_name],
                        num_fewshot=args.num_fewshot,
                        check_integrity=False,
                        gen_kwargs=args.gen_kwargs,
                        task_manager=task_manager,
                        verbosity=str(args.verbosity),
                        predict_only=False,
                        eval_logger=eval_logger,
                        metadata=metadata,
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
                eval_logger.debug(
                    "Passed `--trust_remote_code`, setting environment variable `HF_DATASETS_TRUST_REMOTE_CODE=true`"
                )
                # HACK: import datasets and override its HF_DATASETS_TRUST_REMOTE_CODE value internally,
                # because it's already been determined based on the prior env var before launching our
                # script--`datasets` gets imported by lm_eval internally before these lines can update the env.
                import datasets

                datasets.config.HF_DATASETS_TRUST_REMOTE_CODE = True

                # LM Setup
                if isinstance(args.llm_args, dict):
                    args.llm_args["trust_remote_code"] = True
                else:
                    args.llm_args = args.llm_args + ",trust_remote_code=True"

                # NOTE (Nicolas): Let's fullfill here the tokenized_requests and tokenizer_backend args
                # based on the output_type of the task.
                if task_dict[task_name].get_config("output_type") == "generate_until":
                    args.llm_args["tokenized_requests"] = False
                    args.llm_args["tokenizer_backend"] = None
                elif task_dict[task_name].get_config("output_type") == "loglikelihood":
                    args.llm_args["tokenized_requests"] = True
                    args.llm_args["tokenizer_backend"] = "huggingface"

                if args.requester_args.path == "/v1/completions":
                    lm = SamplerCompletionAPI(
                        requester_args=args.requester_args,
                        mongo_client=mongo_client,
                        wf_id=wf_id,
                        **args.llm_args,
                    )
                elif args.requester_args.path == "/v1/chat/completions":
                    lm = SamplerChatCompletionAPI(
                        requester_args=args.requester_args,
                        mongo_client=mongo_client,
                        wf_id=wf_id,
                        **args.llm_args,
                    )
                # LM Setup (end)
                else:
                    raise ApplicationError(
                        "Unsupported path for LLM API",
                        args.requester_args.path,
                        type="UnsupportedPath",
                        non_retryable=True,
                    )
                # first try to load tokenizer then pass it to be used
                ok = await lm.load_tokenizer()
                if (
                    task_dict[task_name].get_config("output_type") != "generate_until"
                    and not ok
                ):
                    eval_logger.error(
                        f"Skipped LM generation for task {task_name} and framework {args.framework}: Tokenizer and/or config not available."
                    )
                else:
                    try:
                        _ = await lmeh_generator.generate_requests(
                            lm=lm,
                            task_dict=task_dict,
                            mongo_client=mongo_client,
                            args=args,
                            system_instruction=args.system_instruction,
                            apply_chat_template=args.apply_chat_template,
                            fewshot_as_multiturn=args.fewshot_as_multiturn,
                            confirm_run_unsafe_code=args.confirm_run_unsafe_code,
                            eval_logger=eval_logger,
                            timeout_handler=timeout_handler,
                        )
                        eval_logger.info("LM generated successfully.")
                    except ApplicationError as e:
                        raise e

    eval_logger.info("Sample Activity done", task_names=task_names)
    return True
