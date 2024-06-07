from abc import ABC
from typing import (
    Any,
    Dict,
    List,
    Optional,
)
import asyncpg
import numpy as np
import datasets
import random
from tqdm import tqdm
from temporalio.exceptions import ApplicationError
from lm_eval.caching.cache import load_from_cache, save_to_cache
from lm_eval.api.task import ConfigurableTask, TaskConfig, ALL_OUTPUT_TYPES
from lm_eval import utils
from lm_eval.api import samplers
from lm_eval.api.instance import Instance, OutputType
from lm_eval.api.metrics import bits_per_byte, mean, weighted_perplexity
from lm_eval.api.registry import (
    AGGREGATION_REGISTRY,
    DEFAULT_METRIC_REGISTRY,
    get_aggregation,
    get_metric,
    get_metric_aggregation,
    is_higher_better,
)
from lm_eval.caching.cache import load_from_cache, save_to_cache
from lm_eval.filters import build_filter_ensemble
from lm_eval.prompts import get_prompt

from app.app import get_app_logger
import pymongo
from bson import ObjectId
from packages.python.lmeh.utils.mongodb import reconstruct_instances 
eval_logger = get_app_logger("sample")


async def get_max_min_ids(postgres_conn: asyncpg.Connection, table_name: str):
    """
    This function connects to a PostgreSQL database and retrieves the min and max ids for each split

    Args:
    uri: The URI of the PostgreSQL database
    table_name: The name of the table in the database

    Returns:
    A dictionary with the min and max ids for each split
    Example:
    {
        'train': {'min': 0, 'max': 100},
        'validation': {'min': 101, 'max': 200},
        'test': {'min': 201, 'max': 300}
    }
    """
    try:
        # Construct the SQL query
        # noinspection SqlNoDataSourceInspection
        sql_query = """
            SELECT
                "__split",
                MIN("__id") AS min_id,
                MAX("__id") AS max_id
            FROM
                "{}"
            GROUP BY
                "__split";
        """.format(table_name)
        eval_logger.debug(f"SQL query:", sql_query=sql_query)

        # Fetch all rows from the result
        rows = await postgres_conn.fetch(sql_query)
        # assert that rows are not empty
        if len(rows) == 0:
            eval_logger.error(f"No rows found in table:", table_name=table_name, sql_query=sql_query)
            raise ApplicationError(f"No rows found in table {table_name}", non_retryable=True)

        _split_ranges = {}
        for row in rows:
            _split_ranges[row[0]] = {'min': row[1], 'max': row[2]}
    except Exception as error:
        eval_logger.error(f"Error while connecting to PostgreSQL:", error=error)
        raise ApplicationError("Error while connecting to PostgreSQL", non_retryable=True)

    return _split_ranges


def get_split_from_ids(_split_ranges: dict, __ids: List[int]):
    """
    This functions take a list of ids, and detect to which range they belong to

    Args:
    _split_ranges: A dictionary with the min and max ids for each split
    Example:
    {
        'train': {'min': 0, 'max': 100},
        'validation': {'min': 101, 'max': 200},
        'test': {'min': 201, 'max': 300}
    }

    __ids: A list of ids
    Example
    [202, 203, 204, 205]

    Returns:
    The split range to which the ids belong to
    Example:
    'test'
    """
    split_ranges = {}
    for k, v in _split_ranges.items():
        split_ranges[k] = set(range(v['min'], v['max'] + 1))

    split_range = []
    for _id in __ids:
        for k, v in split_ranges.items():
            if _id in v:
                split_range.append(k)
                break
    # all ids should belong to a split range
    if len(split_range) != len(__ids):
        eval_logger.error(f"Ids not in split range:", split_range=split_range, __ids=__ids)
        raise ApplicationError("Some ids do not belong to any split range", non_retryable=True)

    # all ids should belong to a unique split range
    if len(set(split_range)) != 1:
        eval_logger.error(f"Ids in more than one split:", __ids=__ids, split_range=split_range)
        raise ApplicationError("Some ids belong to more than one split.", non_retryable=True)

    return list(set(split_range))[0]


class PocketNetworkConfigurableTask(ConfigurableTask):

    def __init__(
        self,
        data_dir=None,
        cache_dir=None,
        download_mode=None,
        config: Optional[dict] = None,
        postgres_conn: Optional[asyncpg.Connection] = None,
    ) -> None:  # TODO no super() call here
        # Get pre-configured attributes
        self._config = self.CONFIG
        self.postgres_conn = postgres_conn
        # Use new configurations if there was no preconfiguration
        if self.config is None:
            self._config = TaskConfig(**config)
        # Overwrite configs
        else:
            if config is not None:
                self._config.__dict__.update(config)

        if self.config is None:
            raise ValueError(
                "Must pass a config to ConfigurableTask, either in cls.CONFIG or `config` kwarg"
            )

        if isinstance(self.config.metadata, dict):
            if "version" in self.config.metadata:
                self.VERSION = self.config.metadata["version"]

        if self.config.output_type is not None:
            if self.config.output_type not in ALL_OUTPUT_TYPES:
                raise ValueError(
                    f"Got invalid output_type '{self.config.output_type}', must be in '{','.join(ALL_OUTPUT_TYPES)}'"
                )
            self.OUTPUT_TYPE = self.config.output_type

        if self.config.dataset_path is not None:
            self.DATASET_PATH = self.config.dataset_path

        if self.config.dataset_name is not None:
            self.DATASET_NAME = self.config.dataset_name

        self._metric_fn_list = {}
        self._metric_fn_kwargs = {}
        self._aggregation_list = {}
        self._higher_is_better = {}

        if self.config.metric_list is None:
            # TODO: handle this in TaskConfig.__post_init__ ?
            _metric_list = DEFAULT_METRIC_REGISTRY[self.config.output_type]

            for metric_name in _metric_list:
                self._metric_fn_list[metric_name] = get_metric(metric_name)
                self._metric_fn_kwargs[metric_name] = {}
                self._aggregation_list[metric_name] = get_metric_aggregation(
                    metric_name
                )
                self._higher_is_better[metric_name] = is_higher_better(metric_name)
        else:
            for metric_config in self.config.metric_list:
                if "metric" not in metric_config:
                    raise ValueError(
                        "'metric' key not provided for an entry in 'metric_list', must be specified!"
                    )
                metric_name = metric_config["metric"]
                kwargs = {
                    key: metric_config[key]
                    for key in metric_config
                    if key
                       not in ["metric", "aggregation", "higher_is_better", "hf_evaluate"]
                }
                hf_evaluate_metric = (
                        "hf_evaluate" in metric_config
                        and metric_config["hf_evaluate"] is True
                )

                if self.config.process_results is not None:
                    self._metric_fn_list[metric_name] = None
                    self._metric_fn_kwargs[metric_name] = {}
                elif callable(metric_name):
                    metric_fn = metric_name.__call__
                    metric_name = metric_name.__name__
                    self._metric_fn_list[metric_name] = metric_fn
                    self._metric_fn_kwargs[metric_name] = kwargs
                else:
                    self._metric_fn_list[metric_name] = get_metric(
                        metric_name, hf_evaluate_metric
                    )
                    self._metric_fn_kwargs[metric_name] = kwargs

                if "aggregation" in metric_config:
                    agg_name = metric_config["aggregation"]
                    if isinstance(agg_name, str):
                        self._aggregation_list[metric_name] = get_aggregation(agg_name)
                    elif callable(agg_name):  # noqa: E721
                        self._aggregation_list[metric_name] = metric_config[
                            "aggregation"
                        ]
                else:
                    INV_AGG_REGISTRY = {v: k for k, v in AGGREGATION_REGISTRY.items()}
                    metric_agg = get_metric_aggregation(metric_name)
                    eval_logger.warning(
                        f"[Task: {self.config.task}] metric {metric_name} is defined, but aggregation is not. "
                        f"using default "
                        f"aggregation={INV_AGG_REGISTRY[metric_agg]}"
                    )
                    self._aggregation_list[metric_name] = metric_agg

                if "higher_is_better" in metric_config:
                    self._higher_is_better[metric_name] = metric_config[
                        "higher_is_better"
                    ]
                else:
                    eval_logger.warning(
                        f"[Task: {self.config.task}] metric {metric_name} is defined, but higher_is_better is not. "
                        f"using default "
                        f"higher_is_better={is_higher_better(metric_name)}"
                    )
                    self._higher_is_better[metric_name] = is_higher_better(metric_name)

        # call this one with await and this will call post_download that is the same done on
        # the original ConfigurableTask from lm_eval.api.task
        # self.download(self.config.dataset_kwargs)

    async def download(self, dataset_kwargs: Optional[Dict[str, Any]] = None) -> None:
        qty = self._config.metadata['pocket_args'].qty
        doc_ids = self.config.metadata['pocket_args'].doc_ids
        blacklist = self._config.metadata['pocket_args'].blacklist
        postgres_uri = self._config.metadata['pocket_args'].postgres_uri
        table_name = self.DATASET_PATH + "--" + self.DATASET_NAME if self.DATASET_NAME else self.DATASET_PATH
        eval_logger.debug(f"table_name:", table_name=table_name)
        # TODO: ASYNC call to get_max_min_ids
        _split_ranges = await get_max_min_ids(table_name=table_name, postgres_conn=self.postgres_conn)
        eval_logger.debug(f"Split ranges:", _split_ranges=_split_ranges)

        # It's necessary to detect which is the split used to test to take the range, and then get random indexes
        if self.config.test_split:
            _split = self.config.test_split
            # validate that the split exists in the _split_ranges
            self.check_split_exist(_split, _split_ranges)
        elif self.config.validation_split:
            _split = self.config.validation_split
            # validate that the split exists in the _split_ranges
            self.check_split_exist(_split, _split_ranges)
        else:
            eval_logger.error(f"Config without splits:", config=self.config)
            raise ApplicationError(
                f"Neither {self.config.test_split} nor {self.config.validation_split} in splits were found in '_split_ranges'. Available splits are {_split_ranges.keys()}",
                non_retryable=True
            )

        _range = _split_ranges[_split]

        if qty < 0:
            indexes = self.get_all_doc_ids(_split, _split_ranges)
        else:
            if doc_ids:
                if _split != get_split_from_ids(_split_ranges, doc_ids):
                    eval_logger.error(f"Doc_ids not in split range used for evaluation:",
                                      doc_ids=doc_ids, _split=_split, range_min=_range['min'], range_max=_range['max']
                                      )
                    raise ApplicationError(
                        f"Doc_ids not in split range used for test used for evaluation: doc_ids: \
                            {doc_ids}, split: {_split}, range_min: {_range['min']}, range_max: {_range['max']}",
                        non_retryable=True
                    )
                indexes = sorted(doc_ids)
            else:
                indexes = self.generate_random_doc_ids(table_name, _split, qty, _range['min'], _range['max'], blacklist)

        where_clause = self.get_SQL_where_clause(indexes, _split, _split_ranges)
        # Construct the full SQL query
        sql_query = f"SELECT * FROM \"{table_name}\" WHERE {where_clause};"
        ds = datasets.Dataset.from_sql(sql_query, con=postgres_uri)
        dataset = datasets.DatasetDict()
        for split in ds.unique("__split"):
            eval_logger.debug(f"Adding split to DatasetDict:", split=split)
            dataset[split] = ds.filter(lambda x: x["__split"] == split)
        self.dataset = dataset.remove_columns(["__id", "__split"])
        # save in config the indexes used to download the dataset
        self._config.metadata['pocket_args'].doc_ids = indexes
        # Update qty to the number of documents downloaded
        self._config.metadata['pocket_args'].qty = len(indexes)

        ###########################################################
        # call the code that was after the download on the __init__
        ###########################################################
        self.post_download()

    def post_download(self):
        self._training_docs = None
        self._fewshot_docs = None

        if self.config.filter_list is not None:
            self._filters = []
            for filter_config in self.config.filter_list:
                filter_name = filter_config["name"]
                filter_functions = filter_config["filter"]
                components = []
                for function in filter_functions:
                    kwargs = {
                        key: function[key] for key in function if key != "function"
                    }
                    components.append([function["function"], kwargs])
                filter_pipeline = build_filter_ensemble(filter_name, components)
                self._filters.append(filter_pipeline)
        else:
            self._filters = [build_filter_ensemble("none", [["take_first", None]])]

        if self.config.use_prompt is not None:
            eval_logger.debug(f"loading prompt {self.config.use_prompt}")
            self.prompt = get_prompt(
                self.config.use_prompt, self.DATASET_PATH, self.DATASET_NAME
            )
        else:
            self.prompt = None

        if self.fewshot_docs() is not None:
            self.sampler = samplers.get_sampler(
                self.config.fewshot_config.get("sampler", "default")
                if self.config.fewshot_config
                else "default"
            )(list(self.fewshot_docs()), self, rnd=random.Random(1234))

        self.task_docs = self.eval_docs

        # Test One Doc
        self.features = list(self.task_docs.features.keys())
        self.multiple_input = 0
        self.multiple_target = 0
        test_doc = self.task_docs[0]
        test_text = self.doc_to_text(test_doc)
        test_target = self.doc_to_target(test_doc)

        if self.config.doc_to_choice is not None:
            test_choice = self.doc_to_choice(test_doc)
            if not isinstance(test_choice, list):
                eval_logger.error("doc_to_choice must return list")
            else:
                num_choice = len(test_choice)

            if isinstance(test_text, int):
                self.multiple_input = num_choice
        else:
            test_choice = None

        if isinstance(test_target, list):
            self.multiple_target = len(test_target)
        else:
            if (isinstance(test_target, int)) and (test_choice is not None):
                test_target = test_choice[test_target]
            else:
                test_target = str(test_target)

        if test_choice is not None:
            check_choices = test_choice
        else:
            check_choices = [test_target]
        if self.config.doc_to_choice is not None:
            for choice in check_choices:
                choice_has_whitespace = True if choice[0].isspace() else False
                delimiter_has_whitespace = (
                    True
                    if self.config.target_delimiter.rstrip()
                       != self.config.target_delimiter
                    else False
                )

                if delimiter_has_whitespace and choice_has_whitespace:
                    eval_logger.debug(
                        f'Both target_delimiter "{self.config.target_delimiter}" and target choice: "{choice}" have whitespace'
                    )
                elif (not delimiter_has_whitespace) and (not choice_has_whitespace):
                    eval_logger.debug(
                        f'Both target_delimiter "{self.config.target_delimiter}" and target choice: "{choice}" do not have whitespace, ignore if the language you are evaluating on does not require/use whitespace'
                    )

    def build_all_requests(
            self,
            *,
            limit=None,
            rank=None,
            world_size=None,
            cache_requests=False,
            rewrite_requests_cache=False,
    ) -> None:
        """Build a set of Instances for a task, and store them in task.instances"""

        # used with caching
        og_limit = limit

        cache_key = f"requests-{self._config.task}-{self.config.num_fewshot}shot-rank{rank}-world_size{world_size}"

        cached_instances = load_from_cache(file_name=cache_key)

        if cache_requests and cached_instances and not rewrite_requests_cache:
            cached_instances = cached_instances[:limit]

            flattened_instances = [
                instance
                for instance_group in cached_instances
                for instance in instance_group
            ]

            self._instances = flattened_instances
            return

        eval_logger.debug(f"Building contexts for {self.config.task} on rank {rank}...")

        instances = []

        # process all documents when caching is specified for simplicity
        if (
                cache_requests
                and (not cached_instances or rewrite_requests_cache)
                and limit is not None
        ):
            limit = None

        doc_id_docs = list(
            self.doc_iterator(rank=rank, limit=limit, world_size=world_size)
        )

        num_docs = len(doc_id_docs)

        for doc_id, doc in tqdm(
                doc_id_docs,
                total=num_docs,
        ):
            # sample fewshot context #TODO: need to offset doc_id by rank now!
            fewshot_ctx = self.fewshot_context(
                doc,
                0 if self.config.num_fewshot is None else self.config.num_fewshot,
            )

            # TODO: we should override self.config.repeats if doing greedy gen so users don't waste time+compute
            pocket_id = self.config.metadata['pocket_args'].doc_ids[doc_id]
            inst = self.construct_requests(
                doc=doc,
                ctx=fewshot_ctx,
                metadata=(self.config["task"], pocket_id, self.config.repeats),
            )

            if not isinstance(inst, list):
                inst = [inst]

            instances.append(inst)

        # now flatten, this is to allow slicing to work with pickles

        sliced_instances = instances[:og_limit]

        flattened_instances = [
            instance
            for instance_group in sliced_instances
            for instance in instance_group
        ]

        self._instances = flattened_instances

        if len(self._instances) == 0:
            raise ValueError("task.build_requests() did not find any docs!")

        if cache_requests and (not cached_instances or rewrite_requests_cache):
            save_to_cache(file_name=cache_key, obj=instances)

    def check_split_exist(self, split: str, _split_ranges: dict):
        """
        This function checks if a self.config.split exists in the keys of _split_ranges
        """
        if split not in _split_ranges.keys():
            eval_logger.error(f"Split not found in _split_ranges:", split=split, _split_ranges=_split_ranges)
            raise ApplicationError(
                f"'{split}' split not found in _split_ranges: {_split_ranges.keys()}",
                non_retryable=True
            )

    def add_string_ids_range(self, split: str, id_list_str: str, _split_ranges: dict):
        """
        This function adds a range of ids to the id_list_str

        Args:
        split: The split for which the range of ids should be added (this is one of self.config.<training|validation|dev>_split)
        id_list_str: A string of ids separated by commas
        _split_ranges: A dictionary with the min and max ids for each split

        Returns:
        id_list_str: A string of ids separated by commas (to be used in a SQL query)
        """
        min_range = _split_ranges[split]['min']
        max_range = _split_ranges[split]['max'] + 1
        eval_logger.debug(f"Adding ids from split range:", split=split, min_range=min_range, max_range=max_range)
        id_list_str += ', '.join(str(id) for id in range(min_range, max_range))
        return id_list_str

    def generate_random_doc_ids(self, table_name: str, _split: str, qty: int, min: int, max: int,
                                blacklist: List[int] = []) -> List[int]:
        """
        This function generates a list of random numbers within a range, excluding some blacklisted numbers
        """
        # check that the quantity of numbers to generate is less than the range
        if qty > (max - min + 1):
            eval_logger.error(f"quantity overflow:", table_name=table_name, _split=_split, qty=qty, range_min=min,
                              range_max=max)
            raise ApplicationError(
                "Quantity of numbers to generate is greater than the range",
                non_retryable=True
            )
        # Generate a list of random numbers within the range [min, max] excluding the blacklist
        ints = set(range(min, max + 1))
        if len(blacklist) > 0:
            original_len = len(ints)
            # Remove the blacklisted numbers
            ints = ints - set(blacklist)
            # Check that the blacklist numbers were removed
            if len(ints) == original_len:
                eval_logger.error(f"Blacklist out of range:", table_name=table_name, _split=_split, range_min=min,
                                  range_max=max, blacklist=blacklist)
                raise ApplicationError(
                    "Blacklist corresponding to '{}' table & '{}' split were not founded in the range: [{}-{}]".format(
                        table_name, _split, min, max),
                    non_retryable=True
                )
        # sorted random numbers
        choices = sorted(np.random.choice(list(ints), qty, replace=False).tolist())
        eval_logger.debug(f"Random numbers generated:", choices=choices)
        return choices


    def get_all_doc_ids(self, _split:str, _split_ranges: dict) -> List[int]:    
        """
        This function returns all the ids for a given split
        """
        min_range = _split_ranges[_split]['min']
        max_range = _split_ranges[_split]['max']+1
        eval_logger.debug(f"Getting all ids from split range:", split=_split, min_range=min_range, max_range=max_range)
        return list(range(min_range, max_range))

    
    def get_SQL_where_clause(self, indexes, _split: str, _split_ranges: dict):
        """
        This function constructs a WHERE clause for a SQL query. Apply the logic detailed in 
        
        """

        id_list_str = ''
        if self.config.test_split:
            self.check_split_exist(self.config.test_split, _split_ranges)
            if _split != self.config.test_split:
                eval_logger.error(f"mismatch test_split:", _split=_split, test_split=self.config.test_split)
                raise ApplicationError(
                    f"_split '{_split}' not equal to test_split '{self.config.test_split}'",
                    non_retryable=True
                )

            id_list_str += ', '.join(str(id) for id in indexes) + ', '

            if self.config.validation_split:
                self.check_split_exist(self.config.validation_split, _split_ranges)
                id_list_str = self.add_string_ids_range(self.config.validation_split, id_list_str, _split_ranges)

            if self.config.training_split:
                self.check_split_exist(self.config.training_split, _split_ranges)
                id_list_str = self.add_string_ids_range(self.config.training_split, id_list_str, _split_ranges)

            if self.config.fewshot_split:
                self.check_split_exist(self.config.fewshot_split, _split_ranges)
                id_list_str = self.add_string_ids_range(self.config.fewshot_split, id_list_str, _split_ranges)

        elif self.config.validation_split:
            self.check_split_exist(self.config.validation_split, _split_ranges)
            if _split != self.config.validation_split:
                eval_logger.error(f"mismatch validation_split:", _split=_split,
                                  validation_split=self.config.validation_split)
                raise ApplicationError(
                    f"_split '{_split}' not equal to validation_split '{self.config.validation_split}'",
                    non_retryable=True
                )
            id_list_str += ', '.join(str(id) for id in indexes) + ', '
            if self.config.training_split:
                self.check_split_exist(self.config.training_split, _split_ranges)
                id_list_str = self.add_string_ids_range(self.config.training_split, id_list_str, _split_ranges)

            if self.config.fewshot_split:
                self.check_split_exist(self.config.fewshot_split, _split_ranges)
                id_list_str = self.add_string_ids_range(self.config.fewshot_split, id_list_str, _split_ranges)
        else:
            eval_logger.error(f"Config without splits:", config=self.config)
            raise ApplicationError(
                "Neither test_split nor validation_split in config, cannot proceed, please check get_SQL_where_clause",
                non_retryable=True                
                )

        # ensure that id_list_str do not end with a comma
        # in case where only one split is used or the last split is used
        if id_list_str.endswith(', '):
            id_list_str = id_list_str[:-2]

        where_clause = f"__id IN ({id_list_str})"

        return where_clause


class EvaluatePocketNetworkConfigurableTask(PocketNetworkConfigurableTask):
    
    def build_all_requests(
            self,
            *,
            task_id: ObjectId,
            mongo_client: pymongo.MongoClient,
            db_name: str = 'pocket-ml-testbench',
            collection: str = 'tasks',
            limit=None,
            rank=None,
            world_size=None,
            cache_requests=False,
            rewrite_requests_cache=False,
    ) -> None:
        """Build a set of Instances for a task, and store them in task.instances"""
        self._instances = reconstruct_instances(task_id=task_id,
                                                client=mongo_client,
                                                db_name=db_name,
                                                collection=collection)
        if len(self._instances) == 0:
            raise ApplicationError("task.build_requests() did not find any docs!", task_id=task_id)