import json
import logging
import random
from collections import defaultdict
from datetime import datetime
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


# adapted from evaluator.py # def simple_evaluate(..) from lm-eval-harness to generate config task
@positional_deprecated
def get_configurable_task(
    tasks: Optional[List[Union[str, dict, object]]] = None,
    num_fewshot: Optional[int] = None,
    check_integrity: bool = False,
    gen_kwargs: Optional[str] = None,
    task_manager: Optional[Union[TaskManager, PocketNetworkTaskManager]] = None,
    verbosity: str = "ERROR",
    predict_only: bool = False,
    eval_logger: Optional[logging.Logger] = None,
    fewshot_random_seed: Optional[int] = None,
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
        gen_kwargs = simple_parse_args_string(gen_kwargs)
        eval_logger.warning(
            "generation_kwargs specified through cli, these settings will update set parameters in yaml tasks. "
            "Ensure 'do_sample=True' for non-greedy decoding!"
        )
        if gen_kwargs == "":
            gen_kwargs = None

    if task_manager is None:
        task_manager = TaskManager(verbosity)

    task_dict = get_task_dict(tasks, task_manager)
    for task_name in task_dict.keys():
        task_obj = task_dict[task_name]
        if isinstance(task_obj, tuple):
            _, task_obj = task_obj
            if task_obj is None:
                continue

        if task_obj.get_config("output_type") == "generate_until":
            if gen_kwargs is not None:
                task_obj.set_config(
                    key="generation_kwargs", value=gen_kwargs, update=True
                )

        if predict_only:
            # log_samples = True
            eval_logger.debug(
                f"Processing {task_name} in output-only mode. Metrics will not be calculated!"
            )
            # we have to change the class properties post-hoc. This is pretty hacky.
            task_obj.override_metric(metric_name="bypass")

        # override tasks' fewshot values to the provided num_fewshot arg value
        # except if tasks have it set to 0 manually in their configs--then we should never overwrite that
        if num_fewshot is not None:
            if (default_num_fewshot := task_obj.get_config("num_fewshot")) == 0:
                eval_logger.debug(
                    f"num_fewshot has been set to 0 for {task_name} in its config. Manual configuration will be ignored."
                )
            else:
                eval_logger.info(
                    f"Overwriting default num_fewshot of {task_name} from {default_num_fewshot} to {num_fewshot}"
                )
                task_obj.set_config(key="num_fewshot", value=num_fewshot)
        else:
            # if num_fewshot not provided, and the task does not define a default one, default to 0
            if (default_num_fewshot := task_obj.get_config("num_fewshot")) is None:
                task_obj.set_config(key="num_fewshot", value=0)
        if fewshot_random_seed is None:
            fewshot_random_seed = random.randint(0, 2**32 - 1)
        # fewshot_random_seed set for tasks, even with a default num_fewshot (e.g. in the YAML file)
        task_obj.set_fewshot_seed(seed=fewshot_random_seed)
        eval_logger.info(
            f"Setting fewshot random generator seed to {fewshot_random_seed}"
        )

    if check_integrity:
        run_task_tests(task_list=tasks)

    return task_dict


async def generate_requests(
    lm: "LM",
    task_dict,
    mongo_client: AsyncIOMotorClient,
    args: PocketNetworkTaskRequest,
    limit: Optional[int] = None,
    cache_requests: bool = False,
    rewrite_requests_cache: bool = False,
    write_out: bool = False,
    log_samples: bool = True,
    system_instruction: Optional[str] = None,
    apply_chat_template: bool = False,
    fewshot_as_multiturn: bool = False,
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

    for task_output in eval_tasks:
        task: Task = task_output.task
        limit = get_sample_size(task, limit)
        task.build_all_requests(
            limit=limit,
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
                decode = (
                    lm.max_gen_toks if instance.request_type == "generate_until" else 2
                )
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
    cache_requests: bool = False,
    rewrite_requests_cache: bool = False,
    log_samples: bool = True,
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

    for task_output in eval_tasks:
        task: Task = task_output.task
        limit = get_sample_size(task, limit)
        try:
            await task.build_all_requests(
                task_id=task_id,
                mongo_client=mongo_client,
                limit=limit,
                rank=lm.rank,
                world_size=lm.world_size,
                cache_requests=cache_requests,
                rewrite_requests_cache=rewrite_requests_cache,
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
                    result_time=datetime.today().isoformat(),
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
                    result_time=datetime.today().isoformat(),
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

    RANK = lm.rank
    WORLD_SIZE = lm.world_size
    insert_mongo_results = []
    ### Postprocess outputs ###
    # TODO: del model here, maybe (idea: allow user to specify device of e.g. reward model separately)
    for task_output in eval_tasks:
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
            doc_iterator = task.doc_iterator(
                rank=RANK, limit=limit, world_size=WORLD_SIZE
            )
            for i, doc in doc_iterator:
                doc_id = list_doc_id[i]
                result_num_samples.add(doc_id)
                requests = instances_by_doc_id[doc_id]
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
                        "doc_id": doc_id,
                        "doc": doc,
                        "target": target,
                        "arguments": [req.args for req in requests],
                        "resps": [req.resps for req in requests],
                        "filtered_resps": [
                            req.filtered_resps[filter_key] for req in requests
                        ],
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
                            id=doc_id,
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
            result_time=datetime.today().isoformat(),
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
