import json
import logging
from collections import defaultdict
from typing import TYPE_CHECKING, List, Optional, Union

from lm_eval.evaluator_utils import (
    get_sample_size,
    get_task_list,
    run_task_tests,
)
from lm_eval.tasks import TaskManager, get_task_dict
from lm_eval.utils import (
    handle_non_serializable,
    hash_string,
    positional_deprecated,
    simple_parse_args_string,
)

import numpy as np

if TYPE_CHECKING:
    from lm_eval.api.model import LM
    from lm_eval.tasks import Task

from bson import ObjectId
from motor.motor_asyncio import AsyncIOMotorClient
from pymongo import UpdateOne
from temporalio.exceptions import ApplicationError

from packages.python.common.mongodb import MongoClient
from packages.python.lmeh.pocket_lm_eval.tasks import PocketNetworkTaskManager
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.protocol.protocol import (
    NumericSample,
    PocketNetworkMongoDBPrompt,
    PocketNetworkMongoDBResultBase,
    PocketNetworkMongoDBResultNumerical,
    PocketNetworkMongoDBTask,
    PocketNetworkTaskRequest,
    TimeoutHandler,
)


def validate_task_output_type(task_dict, logger):
    """
    Function to avoid running tasks with output_type other than 'generate_until'.
    """
    for task_name, task_obj in task_dict.items():
        if task_obj.get_config("output_type") != "generate_until":
            logger.error(
                f"Task {task_name} has output_type {task_obj.get_config('output_type')}. Currently only 'generate_until' output_type is supported"
            )
            raise ApplicationError(
                f"Task {task_name} has output_type {task_obj.get_config('output_type')}. Currently only 'generate_until' output_type is supported",
                non_retryable=True,
            )
    return


# adapted from evaluator.py # def simple_evaluate(..) from lm-eval-harness to generate config task
@positional_deprecated
def get_configurable_task(
    tasks: Optional[List[Union[str, dict, object]]] = None,
    num_fewshot: Optional[int] = None,
    check_integrity: bool = False,
    gen_kwargs: Union[str, dict, None] = None,
    task_manager: Optional[Union[TaskManager, PocketNetworkTaskManager]] = None,
    verbosity: str = "ERROR",
    predict_only: bool = False,
    eval_logger: Optional[logging.Logger] = None,
    fewshot_random_seed: Optional[int] = None,
    metadata: Optional[dict] = None,
):
    """Instantiate and evaluate a model on a list of tasks.

    :param tasks: list[Union[str, dict, Task]]
        List of task names or Task objects. Task objects will be taken to have name task.EVAL_HARNESS_NAME if defined and type(task).__name__ otherwise.
    :param num_fewshot: int
        Number of examples in few-shot context
    :param check_integrity: bool
        Whether to run the relevant part of the test suite for the tasks
    :param gen_kwargs: str
        String arguments for model generation
        Ignored for all tasks with loglikelihood output_type
    :param predict_only: bool
        If true only model outputs will be generated and returned. Metrics will not be evaluated

    :return
        Task dictionary
    """

    seed_message = []

    if seed_message:
        eval_logger.debug(" | ".join(seed_message))

    if tasks is None:
        tasks = []
    if len(tasks) == 0:
        raise ApplicationError(
            "No tasks specified, or no tasks found. Please verify the task names.",
            non_retryable=True,
        )

    if gen_kwargs is not None:
        if isinstance(gen_kwargs, str):
            gen_kwargs = simple_parse_args_string(gen_kwargs)
        eval_logger.warning(
            f"generation_kwargs: {gen_kwargs} specified through cli, these settings will update set parameters in yaml tasks. "
            "Ensure 'do_sample=True' for non-greedy decoding!"
        )
        if not gen_kwargs:
            gen_kwargs = None

    if task_manager is None:
        # metadata = (
        #     simple_parse_args_string(llm_args)
        #     if isinstance(llm_args, str)
        #     else llm_args
        #     if isinstance(llm_args, dict)
        #     else {}
        # ) | (metadata or {})
        task_manager = TaskManager(metadata=metadata)

    task_dict = get_task_dict(
        tasks,
        task_manager,
    )

    # helper function to recursively apply config overrides to leaf subtasks, skipping their constituent groups.
    # (setting of num_fewshot ; bypassing metric calculation ; setting fewshot seed)
    def _adjust_config(task_dict):
        adjusted_task_dict = {}
        for task_name, task_obj in task_dict.items():
            if isinstance(task_obj, dict):
                adjusted_task_dict = {
                    **adjusted_task_dict,
                    **{task_name: _adjust_config(task_obj)},
                }

            else:
                if task_obj.get_config("output_type") == "generate_until":
                    if gen_kwargs is not None:
                        task_obj.set_config(
                            key="generation_kwargs", value=gen_kwargs, update=True
                        )
                    eval_logger.info(
                        f"{task_obj.config.task}: Using gen_kwargs: {task_obj.config.generation_kwargs}"
                    )

                if predict_only:
                    eval_logger.info(
                        f"Processing {task_name} in output-only mode. Metrics will not be calculated!"
                    )
                    # we have to change the class properties post-hoc. This is pretty hacky.
                    task_obj.override_metric(metric_name="bypass")

                # override tasks' fewshot values to the provided num_fewshot arg value
                # except if tasks have it set to 0 manually in their configs--then we should never overwrite that
                if num_fewshot is not None:
                    if (default_num_fewshot := task_obj.get_config("num_fewshot")) == 0:
                        eval_logger.info(
                            f"num_fewshot has been set to 0 for {task_name} in its config. Manual configuration will be ignored."
                        )
                    else:
                        eval_logger.warning(
                            f"Overwriting default num_fewshot of {task_name} from {default_num_fewshot} to {num_fewshot}"
                        )
                        task_obj.set_config(key="num_fewshot", value=num_fewshot)
                else:
                    # if num_fewshot not provided, and the task does not define a default one, default to 0
                    if (
                        default_num_fewshot := task_obj.get_config("num_fewshot")
                    ) is None:
                        task_obj.set_config(key="num_fewshot", value=0)
                # fewshot_random_seed set for tasks, even with a default num_fewshot (e.g. in the YAML file)
                task_obj.set_fewshot_seed(seed=fewshot_random_seed)
                adjusted_task_dict[task_name] = task_obj

        return adjusted_task_dict

    task_dict = _adjust_config(task_dict)

    # validate task output type
    validate_task_output_type(task_dict, eval_logger)

    if check_integrity:
        run_task_tests(task_list=tasks)

    return task_dict


async def generate_requests(
    lm: "LM",
    task_dict,
    mongo_client: AsyncIOMotorClient,
    args: PocketNetworkTaskRequest,
    limit: Optional[int] = None,
    samples: Optional[dict] = None,
    cache_requests: bool = False,
    rewrite_requests_cache: bool = False,
    write_out: bool = False,
    log_samples: bool = True,
    system_instruction: Optional[str] = None,
    apply_chat_template: bool = False,
    fewshot_as_multiturn: bool = False,
    confirm_run_unsafe_code: bool = False,
    eval_logger: Optional[logging.Logger] = None,
    timeout_handler=TimeoutHandler,
):
    """Generate and save in mongoDB: Task->Instances->Prompts

        :param eval_logger:
    :param rewrite_requests_cache:
    :param cache_requests:
    :param args:
    :param mongo_client:
    :param lm: LM
        Language model to create requests
    :param task_dict: dict[str, Task]
        Dictionary of tasks. Tasks will be taken to have name type(task).config.task .
    :param limit: int, optional
        Limit the number of examples per task (only use this for testing)
    :param write_out: bool
        If True, write out an example document and model input for checking task integrity
    :param log_samples: bool
        If True, write out all model outputs and documents for per-sample measurement and post-hoc analysis
    :param system_instruction: str
        System instruction to be applied to the prompt
    :param apply_chat_template: bool
        If True, apply chat template to the prompt
    :param fewshot_as_multiturn: bool
        Whether to provide the fewshot examples as a multiturn conversation or a single user turn.
    :return
        Dictionary of results
    """

    if limit is not None and samples is not None:
        raise ValueError(
            "Either 'limit' or 'samples' must be None, but both are not None."
        )
    if samples is not None:
        eval_logger.info(f"Evaluating examples for tasks {list(samples.keys())}")

    # tracks all Instances/requests a model must generate output on.
    requests = defaultdict(list)

    # get lists of group hierarchy and each type of request
    eval_tasks = get_task_list(task_dict)
    if not log_samples:
        if not all(
            "bypass" not in getattr(task_output.task, "_metric_fn_list", {}).keys()
            for task_output in eval_tasks
        ):
            raise ValueError("log_samples must be True for 'bypass' metric-only tasks")

    # validation checks:
    # 1.are we running multimodal task <-> non-multimodal model class, or vice-versa.
    # 2.are we running code that is marked as unsafe.
    incompatible_tasks = []
    for task_output in eval_tasks:
        task: Task = task_output.task

        if getattr(task, "MULTIMODAL", False) and not getattr(lm, "MULTIMODAL", False):
            incompatible_tasks.append(task_output.task_name)
        elif getattr(task, "UNSAFE_CODE", False) and not confirm_run_unsafe_code:
            eval_logger.error(
                f"Attempted to run task: {task_output.task_name} which is marked as unsafe. Set confirm_run_unsafe_code=True to run this task.",
                task_name=task_output.task_name,
            )
            raise ApplicationError(
                f"Attempted to run task: {task_output.task_name} which is marked as unsafe. Set confirm_run_unsafe_code=True to run this task.",
                non_retryable=True,
            )
    if len(incompatible_tasks) > 0:
        if not getattr(lm, "MULTIMODAL", False):
            eval_logger.error(
                f"Attempted to run tasks: {incompatible_tasks} which require multimodal input, but the selected model type does not currently implement this. Multimodal support is currently restricted to the ['hf-multimodal', 'vllm-vlm'] model type.",
                task_names=incompatible_tasks,
            )
            raise ApplicationError(
                f"Attempted to run tasks: {incompatible_tasks} which require multimodal input, but the selected model type does not currently implement this. Multimodal support is currently restricted to the ['hf-multimodal', 'vllm-vlm'] model type."
            )
    # end validation check

    # Cache the limit arg.
    limit_arg = limit
    limits = []

    for task_output in eval_tasks:
        task: Task = task_output.task

        limit = get_sample_size(task, limit_arg)
        limits.append(limit)
        task.build_all_requests(
            limit=limit,
            samples=samples.get(task_output.task_name, None)
            if samples is not None
            else samples,
            rank=lm.rank,
            world_size=lm.world_size,
            cache_requests=cache_requests,
            rewrite_requests_cache=rewrite_requests_cache,
            system_instruction=system_instruction,
            apply_chat_template=apply_chat_template,
            fewshot_as_multiturn=fewshot_as_multiturn,
            chat_template=getattr(lm, "apply_chat_template")
            if apply_chat_template
            else None,
            tokenizer_name=getattr(lm, "tokenizer_name", "")
            if apply_chat_template
            else "",
        )
        eval_logger.debug(
            f"Task: {task_output.task_name}; number of requests on this rank: {len(task.instances)}"
        )
        # aggregate Instances by LM method requested to get output.
        for instance in task.instances:
            reqtype = instance.request_type
            requests[reqtype].append(instance)

        ############################################################
        # START: POCKET NETWORK CODE
        ############################################################
        # Verify that all request id are in task.config.metadata due to ConfigurableTask was modified.
        for _, rs in requests.items():
            for r in rs:
                task_name, instance_id = r.metadata[0], r.doc_id
                if (
                    instance_id
                    not in task_dict[task_name].config.metadata["pocket_args"].doc_ids
                ):
                    # noinspection PyArgumentList
                    eval_logger.error(
                        'Instance id not found in task.config.metadata["pocket_args"].doc_ids',
                        instance_id=instance_id,
                        task=task_name,
                        task_ids=task_dict[task_name]
                        .config.metadata["pocket_args"]
                        .doc_ids,
                    )
                    raise ApplicationError(
                        f"Request id {instance_id} not found in task.config.metadata",
                        instance_id,
                        task_name,
                        type="InstanceNotFound",
                        non_retryable=True,
                    )
        ############################################################
        # END: POCKET NETWORK CODE
        ############################################################

    eval_logger.debug("Instances generated successfully:")
    # Run LM on inputs, get all outputs ###
    # execute each type of request
    for reqtype, reqs in requests.items():
        eval_logger.debug(f"Running {reqtype} requests")
        # create `K` copies of each request `req` based off `K = req.repeats`
        cloned_reqs = []
        for req in reqs:
            cloned_reqs.extend([req] * req.repeats)

        # run requests through a model
        resps = getattr(lm, reqtype)(cloned_reqs)

        # put responses from a model into a list of length K for each request.
        for x, req in zip(resps, cloned_reqs):
            req.resps.append(x)

    ############################################################
    # START: POCKET NETWORK CODE
    ############################################################
    insert_mongo_prompts = []
    insert_mongo_tasks = []
    insert_mongo_instances = []
    for task_output in eval_tasks:
        # Task
        task = task_output.task
        instances = task.instances

        task_mongodb = PocketNetworkMongoDBTask(
            **{
                **args.model_dump(),
                **{"total_instances": len(instances), "request_type": task.OUTPUT_TYPE},
            },
        )
        insert_mongo_tasks.append(task_mongodb.model_dump(by_alias=True))
        # Instances
        for instance in instances:
            instance_mongo = MongoOperator.instance_to_dict(
                instance=instance, task_id=task_mongodb.id
            )
            insert_mongo_instances.append(instance_mongo)
            # noinspection PyArgumentList
            # Prompts
            for pocket_req in instance.resps:
                instance_id = instance_mongo["_id"]
                data = pocket_req.model_dump_json(
                    exclude_defaults=True,
                    exclude={"ctxlen", "context_enc", "continuation_enc"},
                )
                # Timeout
                prefill = pocket_req.ctxlen
                if instance.request_type == "generate_until":
                    # if generate_until, we need to get the decode length
                    # try first with the args, then with the task config#
                    # an finally with the default value from the LM
                    gen_kwargs = args.gen_kwargs or task_output.task_config.get(
                        "generation_kwargs", {}
                    )
                    # if empty, try to get from the LM
                    decode = gen_kwargs.get("max_gen_toks", lm.max_gen_toks)
                else:
                    decode = 2
                timeout = int(
                    timeout_handler.get_timeout(prefill=prefill, decode=decode)
                )
                eval_logger.debug(
                    "Timeout:",
                    timeout=timeout,
                    prefill=pocket_req.ctxlen,
                    decode=decode,
                    request_type=instance.request_type,
                )
                # Prompt
                prompt_mongo = PocketNetworkMongoDBPrompt(
                    data=data,
                    task_id=task_mongodb.id,
                    instance_id=instance_id,
                    ctxlen=pocket_req.ctxlen,
                    context_enc=pocket_req.context_enc,
                    continuation_enc=pocket_req.continuation_enc,
                    timeout=timeout,
                )
                insert_mongo_prompts.append(prompt_mongo.model_dump(by_alias=True))
    try:
        async with mongo_client.start_transaction() as session:
            await mongo_client.db["tasks"].insert_many(
                insert_mongo_tasks,
                ordered=False,
                session=session,
            )
            await mongo_client.db["instances"].insert_many(
                insert_mongo_instances,
                ordered=False,
                session=session,
            )
            await mongo_client.db["prompts"].insert_many(
                insert_mongo_prompts,
                ordered=False,
                session=session,
            )

    except Exception as e:
        # noinspection PyArgumentList
        eval_logger.error("Failed to save documents to MongoDB.", error=e)
        raise ApplicationError(
            "Failed to save documents to MongoDB.",
            str(e),
            type="Mongodb",
            non_retryable=True,
        )
    ############################################################
    # END: POCKET NETWORK CODE
    ############################################################
    return True


async def evaluate(
    lm: "LM",
    task_dict,
    task_id: ObjectId,
    mongo_client: MongoClient,
    selected_filters: List[str],
    selected_metrics: List[str],
    limit: Optional[int] = None,
    samples: Optional[dict] = None,
    cache_requests: bool = False,
    rewrite_requests_cache: bool = False,
    log_samples: bool = True,
    system_instruction: Optional[str] = None,
    apply_chat_template: Union[bool, str] = False,
    fewshot_as_multiturn: bool = False,
    confirm_run_unsafe_code: bool = False,
    eval_logger: Optional[logging.Logger] = None,
):
    """
    :param lm: LM
        Language model to retrieve requests
    :param task_dict: dict[str, Task]
        Dictionary of tasks. Tasks will be taken to have name type(task).config.task .
    :param limit: int, optional
        Limit the number of examples per task (only use this for testing)
    :param write_out: bool
        If True, write out an example document and model input for checking task integrity
    :param log_samples: bool
        If True, write out all model outputs and documents for per-sample measurement and post-hoc analysis
    :param system_instruction: str
        System instruction to be applied to the prompt
    :param apply_chat_template: Union[bool, str]
        Specifies whether to apply a chat template to the prompt.
        - If set to True, the default chat template is applied.
        - If set to a string, applies the specified chat template by name.
        Defaults to False (no chat template applied).
    :param fewshot_as_multiturn: bool
        Whether to provide the fewshot examples as a multiturn conversation or a single user turn.
    :return
        Dictionary of results
    """

    async def save_results(
        mongo_client: MongoClient,
        insert_mongo_results: List[dict],
        eval_logger: Optional[logging.Logger] = None,
    ):
        try:
            async with mongo_client.start_transaction() as session:
                bulk_op = []
                bulk_task_op = []

                for result in insert_mongo_results:
                    if "_id" in result.keys():
                        result.pop(
                            "_id"
                        )  # TODO: Find out how this arrives here on some occasions... This should not be here I think...
                    bulk_op.append(
                        UpdateOne(
                            filter={
                                "result_data.task_id": result["result_data"]["task_id"]
                            },
                            update={"$set": result},
                            upsert=True,
                        )
                    )
                    bulk_task_op.append(
                        UpdateOne(
                            filter={"_id": result["result_data"]["task_id"]},
                            update={"$set": {"evaluated": True}},
                        )
                    )

                await mongo_client.db["results"].bulk_write(
                    bulk_op,
                    ordered=False,
                    session=session,
                )
                await mongo_client.db["tasks"].bulk_write(
                    bulk_task_op,
                    ordered=False,
                    session=session,
                )
        except Exception as e:
            eval_logger.debug(
                "Documents that failed to insert:",
                insert_mongo_results=insert_mongo_results,
            )
            eval_logger.error("Failed to save documents (results) to MongoDB.", error=e)
            raise ApplicationError(
                "Failed to save documents (results) to MongoDB.",
                str(e),
                type="Mongodb",
                non_retryable=True,
            )
        return

    if apply_chat_template:
        eval_logger.warning(
            "Chat template formatting change affects loglikelihood and multiple-choice tasks. See docs/chat-template-readme.md for details."
        )
    # tracks all Instances/requests a model must generate output on.
    requests = defaultdict(list)

    # get lists of group hierarchy and each type of request
    eval_tasks = get_task_list(task_dict)
    if not log_samples:
        if not all(
            "bypass" not in getattr(task_output.task, "_metric_fn_list", {}).keys()
            for task_output in eval_tasks
        ):
            raise ValueError("log_samples must be True for 'bypass' metric-only tasks")

    # validation checks:
    # 1.are we running multimodal task <-> non-multimodal model class, or vice-versa.
    # 2.are we running code that is marked as unsafe.
    incompatible_tasks = []
    for task_output in eval_tasks:
        task: Task = task_output.task

        if getattr(task, "MULTIMODAL", False) and not getattr(lm, "MULTIMODAL", False):
            incompatible_tasks.append(task_output.task_name)
        elif getattr(task, "UNSAFE_CODE", False) and not confirm_run_unsafe_code:
            eval_logger.error(
                f"Attempted to run task: {task_output.task_name} which is marked as unsafe. Set confirm_run_unsafe_code=True to run this task.",
                task_name=task_output.task_name,
                task_id=str(task_id),
            )
            raise ApplicationError(
                f"Attempted to run task: {task_output.task_name} which is marked as unsafe. Set confirm_run_unsafe_code=True to run this task.",
                non_retryable=True,
            )
    if len(incompatible_tasks) > 0:
        if not getattr(lm, "MULTIMODAL", False):
            eval_logger.error(
                f"Attempted to run tasks: {incompatible_tasks} which require multimodal input, but the selected model type does not currently implement this. Multimodal support is currently restricted to the ['hf-multimodal', 'vllm-vlm'] model type.",
                task_names=incompatible_tasks,
                task_id=str(task_id),
            )
            raise ApplicationError(
                f"Attempted to run tasks: {incompatible_tasks} which require multimodal input, but the selected model type does not currently implement this. Multimodal support is currently restricted to the ['hf-multimodal', 'vllm-vlm'] model type."
            )
    # end validation check

    # Cache the limit arg.
    limit_arg = limit
    limits = []

    for task_output in eval_tasks:
        task: Task = task_output.task
        limit = get_sample_size(task, limit_arg)
        limits.append(limit)
        try:
            await task.build_all_requests(
                task_id=task_id,
                mongo_client=mongo_client,
                limit=limit,
                rank=lm.rank,
                world_size=lm.world_size,
                cache_requests=cache_requests,
                rewrite_requests_cache=rewrite_requests_cache,
                system_instruction=system_instruction,
                apply_chat_template=bool(apply_chat_template),
                fewshot_as_multiturn=fewshot_as_multiturn,
                chat_template=getattr(lm, "apply_chat_template")
                if apply_chat_template
                else None,
                tokenizer_name=getattr(lm, "tokenizer_name", "")
                if apply_chat_template
                else "",
            )
            eval_logger.debug(
                f"Task: {task_output.task_name}; number of requests on this rank: {len(task.instances)}"
            )
            # aggregate Instances by LM method requested to get output.
            for instance in task.instances:
                reqtype = instance.request_type
                requests[reqtype].append(instance)
        except Exception as e:
            raise e

        if len(task.instances) == 0:
            insert_mongo_results = []
            if len(task.failed_instances) == 0:
                # Nothing to do, not sure this state is reachable
                eval_logger.debug(
                    "No instances/doc_id generated for task.", task_id=str(task_id)
                )
                base_result = PocketNetworkMongoDBResultBase(
                    task_id=task_id,
                    status=1,
                    num_samples=0,
                    result_height=task.result_height,
                )
                num_result = PocketNetworkMongoDBResultNumerical(
                    result_data=base_result, scores=[]
                )
            else:
                # Just add all failed instances
                scores = []
                for instance in task.failed_instances:
                    numericSample = NumericSample(
                        score=0.0,
                        run_time=0.0,
                        id=instance["id"],
                        status_code=instance["code"],
                        error_str=instance["error"],
                    )
                    scores.append(numericSample)

                base_result = PocketNetworkMongoDBResultBase(
                    task_id=task_id,
                    status=0,
                    num_samples=len(task.failed_instances),
                    result_height=task.result_height,
                )
                num_result = PocketNetworkMongoDBResultNumerical(
                    result_data=base_result, scores=scores
                )

            # Save to DB and return
            insert_mongo_results.append(num_result.model_dump(by_alias=True))
            eval_logger.debug("Mongo Result:", mongo_result=insert_mongo_results)

            await save_results(
                mongo_client=mongo_client,
                insert_mongo_results=insert_mongo_results,
                eval_logger=eval_logger,
            )
            return True

    eval_logger.debug("Instances generated successfully:")
    ### Run LM on inputs, get all outputs ###
    # execute each type of request
    try:
        for reqtype, reqs in requests.items():
            eval_logger.debug(f"Running {reqtype} requests")
            # create `K` copies of each request `req` based off `K = req.repeats`
            cloned_reqs = []
            for req in reqs:
                cloned_reqs.extend([req] * req.repeats)
                req.times = []
            # run requests through model
            resps = getattr(lm, reqtype)(cloned_reqs)
            # Get times POKT Network
            times = getattr(lm, "response_times")(cloned_reqs)

            # put responses from model into a list of length K for each request.
            for x, t, req in zip(resps, times, cloned_reqs):
                req.resps.append(x)
                req.times.append(t)
    except Exception as e:
        eval_logger.error(
            "Failed to process response from the LM model.",
            error=e,
            request_type=reqtype,
            requests=reqs,
        )
        raise ApplicationError(
            "Failed to process response from the LM model.",
            str(e),
            type="LMResponse",
            non_retryable=True,
        )

    RANK = lm.rank
    WORLD_SIZE = lm.world_size
    insert_mongo_results = []
    ### Postprocess outputs ###
    # TODO: del model here, maybe (idea: allow user to specify device of e.g. reward model separately)
    for task_output, limit in zip(eval_tasks, limits):
        task = task_output.task
        task.apply_filters()
        ### Collect values of metrics on all datapoints ###
        # # unpack results and sort back in order and return control to Task
        # TODO: make it possible to use a different metric per filter
        # Pre-process task.instances to group by doc_id
        instances_by_doc_id = defaultdict(list)
        for instance in task.instances:
            instances_by_doc_id[instance.doc_id].append(instance)
        list_doc_id = list(instances_by_doc_id.keys())
        # Sort instances within each group
        for instances in instances_by_doc_id.values():
            instances.sort(key=lambda x: x.idx)
        # iterate over different filters used
        scores = []
        result_num_samples = set()
        for filter_key in task.instances[0].filtered_resps.keys():
            if filter_key not in selected_filters:
                eval_logger.warning(
                    "Skipping Filter Key. This can signal misconfiguration of task in `task_config.py`",
                    filter_key=filter_key,
                )
                continue
            eval_logger.debug("Entering Filter Key:", filter_key=filter_key)
            indices = (
                samples.get(task_output.task_name, None)
                if samples is not None
                else None
            )
            doc_iterator = task.doc_iterator(
                rank=RANK,
                limit=limit,
                world_size=WORLD_SIZE,
                samples=indices,
            )
            for i, doc in doc_iterator:
                if indices:
                    doc_id_true = indices[i]
                else:
                    doc_id_true = list_doc_id[i]
                result_num_samples.add(doc_id_true)
                requests = instances_by_doc_id[doc_id_true]
                try:
                    if "kwargs" in doc.keys():
                        # Make sure the kwargs are a dict not a string
                        doc["kwargs"] = [json.loads(a) for a in doc["kwargs"]]

                    metrics = task.process_results(
                        doc, [req.filtered_resps[filter_key] for req in requests]
                    )
                except Exception as e:
                    eval_logger.debug(
                        "task.process_results inputs",
                        doc=doc,
                        responses=[req.filtered_resps[filter_key] for req in requests],
                    )
                    eval_logger.error("Failed to process results in LMEH.", error=e)
                    raise ApplicationError(
                        "Failed process results.",
                        str(e),
                        type="LMEH",
                        non_retryable=True,
                    )

                response_times = [np.mean(req.times).astype(float) for req in requests]
                if log_samples:
                    target = task.doc_to_target(doc)
                    example = {
                        "doc_id": doc_id_true,
                        "doc": doc,
                        "target": target,
                        "arguments": [req.args for req in requests],
                        "resps": [req.resps for req in requests],
                        "filtered_resps": [
                            req.filtered_resps[filter_key] for req in requests
                        ],
                        "filter": filter_key,
                        "metrics": list(metrics.keys()),
                        "doc_hash": hash_string(
                            json.dumps(
                                requests[0].doc,
                                indent=2,
                                default=handle_non_serializable,
                                ensure_ascii=False,
                            )
                        ),
                        "prompt_hash": hash_string(requests[0].arguments[0]),
                        "target_hash": hash_string(str(target)),
                    }
                    example.update(metrics)
                    task_output.logged_samples.append(example)
                for (metric, value), ms in zip(metrics.items(), response_times):
                    task_output.sample_metrics[(metric, filter_key)].append(value)
                    if metric in selected_metrics:
                        numericSample = NumericSample(
                            score=example[metric],
                            run_time=ms,
                            id=doc_id_true,
                            status_code=0,
                            error_str="",
                        )
                        scores.append(numericSample)
        # If there are failed samples, add them here to the scores list
        for instance in task.failed_instances:
            numericSample = NumericSample(
                score=0.0,
                run_time=0.0,
                id=instance["id"],
                status_code=instance["code"],
                error_str=instance["error"],
            )
            scores.append(numericSample)

        base_result = PocketNetworkMongoDBResultBase(
            task_id=task_id,
            status=0,
            num_samples=len(result_num_samples) + len(task.failed_instances),
            result_height=task.result_height,
        )
        num_result = PocketNetworkMongoDBResultNumerical(
            result_data=base_result, scores=scores
        )
        insert_mongo_results.append(num_result.model_dump(by_alias=True))
    eval_logger.debug("Mongo Result:", mongo_result=insert_mongo_results)

    await save_results(
        mongo_client=mongo_client,
        insert_mongo_results=insert_mongo_results,
        eval_logger=eval_logger,
    )

    return True
