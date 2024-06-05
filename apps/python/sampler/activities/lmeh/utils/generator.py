import logging
from collections import defaultdict
from typing import TYPE_CHECKING, List, Optional, Union

from lm_eval.evaluator_utils import (
    get_sample_size,
    get_task_list,
    print_writeout,
    run_task_tests,
)
from lm_eval.tasks import TaskManager, get_task_dict
from lm_eval.utils import positional_deprecated, simple_parse_args_string

if TYPE_CHECKING:
    from lm_eval.api.model import LM
    from lm_eval.tasks import Task

from temporalio.exceptions import ApplicationError
from activities.lmeh.utils import mongodb as lmeh_mongodb
from activities.lmeh.utils.pocket_lm_eval.tasks import PocketNetworkTaskManager
from protocol.protocol import PocketNetworkTaskRequest, PocketNetworkMongoDBTask, PocketNetworkMongoDBPrompt
from pymongo import MongoClient


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
            "No tasks specified, or no tasks found. Please verify the task names.", non_retryable=True
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
            log_samples = True
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
                eval_logger.warning(
                    f"Overwriting default num_fewshot of {task_name} from {default_num_fewshot} to {num_fewshot}"
                )
                task_obj.set_config(key="num_fewshot", value=num_fewshot)
        else:
            # if num_fewshot not provided, and the task does not define a default one, default to 0
            if (default_num_fewshot := task_obj.get_config("num_fewshot")) is None:
                task_obj.set_config(key="num_fewshot", value=0)

    if check_integrity:
        run_task_tests(task_list=tasks)

    return task_dict


def genererate_requests(
        lm: "LM",
        task_dict,
        mongo_client: MongoClient,
        args: PocketNetworkTaskRequest,
        limit: Optional[int] = None,
        cache_requests: bool = False,
        rewrite_requests_cache: bool = False,
        write_out: bool = False,
        log_samples: bool = True,
        eval_logger: Optional[logging.Logger] = None,
):
    """Generate and save in mongoDB: Task->Instances->Prompts

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
    :return
        Dictionary of results
    """

    # tracks all Instances/requests a model must generate output on.
    requests = defaultdict(list)

    # get lists of group hierarchy and each type of request
    task_hierarchy, eval_tasks = get_task_list(task_dict)
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
        )
        eval_logger.debug(
            f"Task: {task_output.task_name}; number of requests on this rank: {len(task.instances)}"
        )

        if write_out:
            print_writeout(task)
        # aggregate Instances by LM method requested to get output.
        for instance in task.instances:
            reqtype = instance.request_type
            requests[reqtype].append(instance)

        ############################################################
        # START: POCKET NETWORK CODE
        ############################################################
        # Verify that all request id are in task.config.metadata due to ConfigurableTask was modified.
        for reqtype, rs in requests.items():
            for r in rs:
                task_n, doc_id = r.metadata[0], r.doc_id
                if doc_id not in task_dict[task_n].config.metadata['pocket_args'].doc_ids:
                    eval_logger.error(f"Instance id not found in task.config.metadata[\"pocket_args\"].doc_ids",
                                      instance_id=doc_id, task=task_n,
                                      task_ids=task_dict[task_n].config.metadata['pocket_args'].doc_ids)
                    raise ApplicationError(f"Request id {doc_id} not found in task.config.metadata", non_retryable=True)
        ############################################################
        # END: POCKET NETWORK CODE
        ############################################################

    eval_logger.debug("Instances generated successfully:")
    ### Run LM on inputs, get all outputs ###
    # execute each type of request
    for reqtype, reqs in requests.items():
        eval_logger.debug(f"Running {reqtype} requests")
        # create `K` copies of each request `req` based off `K = req.repeats`
        cloned_reqs = []
        for req in reqs:
            cloned_reqs.extend([req] * req.repeats)

        # run requests through model
        resps = getattr(lm, reqtype)(cloned_reqs)
        eval_logger.debug(f"Response:", resps=resps)

        # put responses from model into a list of length K for each request.
        for x, req in zip(resps, cloned_reqs):
            req.resps.append(x)

    ############################################################
    # START: POCKET NETWORK CODE
    ############################################################
    insert_mongo_prompt = []
    insert_mongo_tasks = []
    insert_mongo_instances = []
    for task_output in eval_tasks:
        # Task
        task = task_output.task
        instances = task.instances

        task_mongodb = PocketNetworkMongoDBTask(**{
            **args.model_dump(),
            **{"total_instances": len(instances),
               "request_type": task.OUTPUT_TYPE}})
        insert_mongo_tasks.append(task_mongodb.model_dump(by_alias=True))
        # Instances
        for instance in instances:
            instance_mongo = lmeh_mongodb.instance_to_dict(instance=instance, task_id=task_mongodb.id)
            insert_mongo_instances.append(instance_mongo)
            eval_logger.debug(f"Instance:", instance=instance)
            # Prompts
            for pocket_req in instance.resps:
                instance_id = instance_mongo['_id']
                data = pocket_req.model_dump_json(exclude_defaults=True)
                prompt_mongo = PocketNetworkMongoDBPrompt(data=data, task_id=task_mongodb.id, instance_id=instance_id)
                insert_mongo_prompt.append(prompt_mongo.model_dump())
                eval_logger.debug(f"Data:", PocketNetworkMongoDBPrompt=prompt_mongo)
    try:
        with mongo_client.start_session() as session:
            with session.start_transaction():
                mongo_client['pocket-ml-testbench']['tasks'].insert_many(insert_mongo_tasks, ordered=False,
                                                                         session=session)
                mongo_client['pocket-ml-testbench']['instances'].insert_many(insert_mongo_instances, ordered=False,
                                                                             session=session)
                mongo_client['pocket-ml-testbench']['prompts'].insert_many(insert_mongo_prompt, ordered=False,
                                                                           session=session)
                eval_logger.debug("Instances saved to MongoDB successfully.")
    except Exception as e:
        eval_logger.error("Failed to save Instances to MongoDB.")
        raise ApplicationError("Failed to save instances to MongoDB.", error=e, non_retryable=True)
    ############################################################
    # END: POCKET NETWORK CODE
    ############################################################
    return True
