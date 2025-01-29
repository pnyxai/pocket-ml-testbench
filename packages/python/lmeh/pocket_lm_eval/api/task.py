import datetime
import decimal
import json
import random
import time
from collections.abc import Callable
from typing import (
    Any,
    Dict,
    List,
    Optional,
    Union,
)

import asyncpg
import numpy as np
from app.app import get_app_logger
from bson import ObjectId
from datasets import Dataset, DatasetDict, DownloadMode, load_dataset
from lm_eval import utils
from lm_eval.api import samplers
from lm_eval.api.registry import (
    AGGREGATION_REGISTRY,
    DEFAULT_METRIC_REGISTRY,
    get_aggregation,
    get_metric,
    get_metric_aggregation,
    is_higher_better,
)
from lm_eval.api.task import ALL_OUTPUT_TYPES, ConfigurableTask, TaskConfig
from lm_eval.caching.cache import load_from_cache, save_to_cache
from lm_eval.filters import build_filter_ensemble
from lm_eval.prompts import get_prompt
from temporalio.exceptions import ApplicationError
from tqdm import tqdm

from packages.python.common.mongodb import MongoClient
from packages.python.lmeh.utils.mongodb import MongoOperator


class SqlDatasetLoader:
    def __init__(self, postgres_connection, table_name, query):
        self.postgres_connection = postgres_connection
        self.table_name = table_name
        self.query = query
        self.dataset = None

    @staticmethod
    def mutate_json_features(doc, json_features):
        for feature_name in json_features:
            doc[feature_name] = json.loads(doc[feature_name])
        return doc

    async def generate_and_fetch_dataset(self):
        records = []

        query = f"select column_name, data_type from information_schema.columns where table_name = '{self.table_name}' AND data_type='json';"
        features_records = await self.postgres_connection.fetch(query)
        json_features = [dict(r)["column_name"] for r in features_records]

        async for record in self.postgres_connection.cursor(self.query, prefetch=5000):
            records.append(
                SqlDatasetLoader.mutate_json_features(dict(record), json_features)
            )

        return Dataset.from_list(records)


class SqlDatasetSaver:
    _ID_NAME = "__id"
    _SPLIT_NAME = "__split"

    POCKET_COLUMNS = {_ID_NAME: "INTEGER", _SPLIT_NAME: "TEXT"}

    PRIMARY_KEY_DEF = f"PRIMARY KEY ({_ID_NAME}, {_SPLIT_NAME})"

    DATA_TYPE_MAPPING = {
        int: "INTEGER",
        bool: "BOOLEAN",
        float: "REAL",
        str: "TEXT",
        datetime.datetime: "TIMESTAMP",
        datetime.date: "DATE",
        datetime.time: "TIME",
        decimal.Decimal: "DECIMAL",
        list: "[]",
        dict: "JSON",
        bytes: "BYTEA",
    }

    def __init__(
        self, table_name, dataset_path, dataset_name, connection, logger, hf_token=None
    ):
        self.table_name = table_name
        self.dataset_path = dataset_path
        self.dataset_name = dataset_name
        self._conn = connection
        self._logger = logger
        self._id = 0
        self.dataset = None
        self.splits = []
        self.split_index = 0
        self.dataset_iterator = None
        self.first_record = None
        self.first_record_consumed = None
        # HuggingFace Token (some datasets might need it)
        self.hf_token = hf_token

    def _calculate_columns_def(self):
        self.columns = {}

        # Add manually k,v pairs "pocket_ID":INT, and "SPLIT":TEXT
        self.columns.update(SqlDatasetSaver.POCKET_COLUMNS)

        for key, value in self.first_record.items():
            if key not in self.columns:
                # If the column doesn't exist yet, infer its data type from the value
                self.columns[key] = self._infer_data_type(value)

        # Generate column definitions
        self.columns_def = [
            f'"{column_name}" {data_type}'
            for column_name, data_type in self.columns.items()
        ]

        # Generate primary key definition
        self.columns_def.append(self.PRIMARY_KEY_DEF)

    def _infer_data_type(self, value: Any):
        v_type = self.DATA_TYPE_MAPPING.get(type(value), "TEXT")
        # Handle lists
        if v_type == "[]":
            subvalue_type = self.DATA_TYPE_MAPPING.get(type(value[0]), "TEXT")
            v_type = subvalue_type + v_type

        return v_type

    async def _prepare_table(self):
        column_definitions_str = ", ".join(self.columns_def)
        create_table = (
            f'CREATE TABLE IF NOT EXISTS "{self.table_name}" ({column_definitions_str})'
        )
        await self._conn.execute(create_table)

    async def transfer(self, dataset_kwargs: Optional[Dict[str, Any]] = None):
        self._logger.info(
            "Starting dataset transfer",
            dataset_path=self.dataset_path,
            dataset_name=self.dataset_name,
            dataset_kwargs=dataset_kwargs,
        )
        start_time = time.perf_counter()
        self.dataset = load_dataset(
            path=self.dataset_path,
            name=self.dataset_name,
            token=self.hf_token,
            **dataset_kwargs if dataset_kwargs is not None else {},
        ).flatten_indices()
        for split in self.dataset:
            self.splits.append(split)
        ds = self.dataset[self.splits[self.split_index]].to_iterable_dataset()
        self.dataset_iterator = iter(ds)
        self.first_record = next(self.dataset_iterator)
        self.first_record_consumed = False
        self._calculate_columns_def()
        # ensure the table is ready to receive records
        await self._prepare_table()
        # start transferring data
        await self._conn.copy_records_to_table(
            self.table_name,
            columns=list(self.columns.keys()),
            records=self,
        )
        end_time = time.perf_counter()
        elapsed_time_ms = end_time - start_time
        self._logger.info(
            "Dataset transfer done",
            dataset_path=self.dataset_path,
            dataset_name=self.dataset_name,
            took=f"{round(elapsed_time_ms, 2)} seconds",
        )

    def _next(self):
        try:
            return next(self.dataset_iterator)
        except StopIteration:
            self.split_index += 1
            if self.split_index < len(self.splits):
                # set as iterator the iterator of the next split
                ds = self.dataset[self.splits[self.split_index]].to_iterable_dataset()
                self.dataset_iterator = iter(ds)
                # call it again
                return self._next()

            raise StopAsyncIteration

    def __aiter__(self):
        return self

    async def __anext__(self):
        try:
            if not self.first_record_consumed and self.split_index == 0:
                # consume the first record and set the flag to keep going
                record = self.first_record
                self.first_record_consumed = True
            else:
                # Convert each record to a PostgreSQL textual format and encode to bytes
                record = self._next()

            # place the values in the order of the columns
            values = []
            for column_name in self.columns:
                if column_name == self._ID_NAME:
                    values.append(self._id)
                elif column_name == self._SPLIT_NAME:
                    values.append(self.splits[self.split_index])
                else:
                    values.append(record[column_name])

            current_row = record.copy()
            current_row[self._ID_NAME] = self._id
            current_row[self._SPLIT_NAME] = self.splits[self.split_index]

            row_to_insert = list()
            for key in self.columns.keys():
                val = current_row.get(key)
                # Convert dict (JSON) to strings
                if isinstance(val, dict):
                    val = json.dumps(val)
                # Also check if the instance is not a list of dicts!
                if isinstance(val, list):
                    n_val = list()
                    for t_val in val:
                        if isinstance(t_val, dict):
                            n_val.append(json.dumps(t_val))
                        else:
                            n_val.append(t_val)
                    val = n_val
                row_to_insert.append(val)

            self._id += 1
            return tuple(row_to_insert)
        except StopIteration:
            raise StopAsyncIteration


class PocketNetworkConfigurableTask(ConfigurableTask):
    def __init__(
        self,
        data_dir: Optional[str] = None,
        cache_dir: Optional[str] = None,
        download_mode: Optional[DownloadMode] = None,
        config: Optional[dict] = None,
        postgres_conn: Optional[asyncpg.Connection] = None,
        eval_logger: Any = get_app_logger("sampler"),
        hf_token: Optional[str] = None,
    ) -> None:  # TODO no super() call here
        self.hf_token = hf_token
        # Get pre-configured attributes
        self._config = self.CONFIG
        self.postgres_conn = postgres_conn
        self.eval_logger = eval_logger
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

        self.TABLE_NAME = (
            self.DATASET_PATH + "--" + self.DATASET_NAME
            if self.DATASET_NAME
            else self.DATASET_PATH
        )
        self.dataset = None
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
                    inv_agg_registry = {v: k for k, v in AGGREGATION_REGISTRY.items()}
                    metric_agg = get_metric_aggregation(metric_name)
                    self.eval_logger.warning(
                        f"[Task: {self.config.task}] metric {metric_name} is defined, but aggregation is not. "
                        f"using default "
                        f"aggregation={inv_agg_registry[metric_agg]}"
                    )
                    self._aggregation_list[metric_name] = metric_agg

                if "higher_is_better" in metric_config:
                    self._higher_is_better[metric_name] = metric_config[
                        "higher_is_better"
                    ]
                else:
                    self.eval_logger.warning(
                        f"[Task: {self.config.task}] metric {metric_name} is defined, but higher_is_better is not. "
                        f"using default "
                        f"higher_is_better={is_higher_better(metric_name)}"
                    )
                    self._higher_is_better[metric_name] = is_higher_better(metric_name)

        # call this one with await, and this will call post_download that is the same done on
        # the original ConfigurableTask from lm_eval.api.task
        # self.download(self.config.dataset_kwargs)

    def get_table_name(self):
        return self.TABLE_NAME

    async def save_to_sql(self) -> None:
        streamer = SqlDatasetSaver(
            table_name=self.TABLE_NAME,
            dataset_path=self._config["dataset_path"],
            dataset_name=self._config["dataset_name"],
            connection=self.postgres_conn,
            hf_token=self.hf_token,
            logger=self.eval_logger,
        )

        # transfer from dataset to the table
        await streamer.transfer(self.config.dataset_kwargs)

    async def load_from_sql(
        self, dataset_kwargs: Optional[Dict[str, Any]] = None
    ) -> None:
        qty = self._config.metadata["pocket_args"].qty
        doc_ids = self.config.metadata["pocket_args"].doc_ids
        blacklist = self._config.metadata["pocket_args"].blacklist
        _split_ranges = await self.get_max_min_ids(
            table_name=self.TABLE_NAME, postgres_conn=self.postgres_conn
        )

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
            self.eval_logger.error("Config without splits:", config=self.config)
            raise ApplicationError(
                f"Neither {self.config.test_split} nor {self.config.validation_split} in splits were found in "
                f"'_split_ranges'. Available splits are {_split_ranges.keys()}",
                non_retryable=True,
            )

        _range = _split_ranges[_split]

        if qty < 0:
            indexes = self.get_all_doc_ids(_split, _split_ranges)
        else:
            if doc_ids:
                if _split != self.get_split_from_ids(_split_ranges, doc_ids):
                    self.eval_logger.error(
                        "Doc_ids not in split range used for evaluation:",
                        doc_ids=doc_ids,
                        _split=_split,
                        range_min=_range["min"],
                        range_max=_range["max"],
                    )
                    raise ApplicationError(
                        f"Doc_ids not in split range used for test used for evaluation: doc_ids: \
                            {doc_ids}, split: {_split}, range_min: {_range['min']}, range_max: {_range['max']}",
                        non_retryable=True,
                    )
                indexes = sorted(doc_ids)
            else:
                indexes = self.generate_random_doc_ids(
                    self.TABLE_NAME,
                    _split,
                    qty,
                    _range["min"],
                    _range["max"],
                    blacklist,
                )

        where_clause = self.get_SQL_where_clause(indexes, _split, _split_ranges)
        # Construct the full SQL query
        sql_query = f'SELECT * FROM "{self.TABLE_NAME}" WHERE {where_clause};'
        self.eval_logger.debug("SQL Query:", sql_query=sql_query)
        ds = await SqlDatasetLoader(
            postgres_connection=self.postgres_conn,
            query=sql_query,
            table_name=self.TABLE_NAME,
        ).generate_and_fetch_dataset()
        # assign dataset as dataset dictionary
        ds_dict = DatasetDict()
        for split in ds.unique("__split"):
            self.eval_logger.debug("Adding split to DatasetDict:", split=split)
            ds_dict[split] = ds.filter(lambda x: x["__split"] == split)
        self.dataset = ds_dict.remove_columns(["__split"])
        # save in config the indexes used to download the dataset
        self._config.metadata["pocket_args"].doc_ids = indexes
        # Update qty to the number of documents downloaded
        self._config.metadata["pocket_args"].qty = len(indexes)
        self.eval_split = _split
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
            self.eval_logger.debug(f"loading prompt {self.config.use_prompt}")
            self.prompt = get_prompt(
                self.config.use_prompt, self.DATASET_PATH, self.DATASET_NAME
            )
        else:
            self.prompt = None

        if self.fewshot_docs() is not None:
            self.fewshot_rnd = (
                random.Random()
            )  # setting with no seed, to be overridden at a later time
            config_sampler: Union[str, Callable] = (
                self.config.fewshot_config.get("sampler", "default")
                if self.config.fewshot_config
                else "default"
            )
            if isinstance(config_sampler, str):
                self.sampler = samplers.get_sampler(config_sampler)(
                    list(self.fewshot_docs()), self, rnd=self.fewshot_rnd
                )
            elif callable(config_sampler) and issubclass(
                config_sampler, samplers.ContextSampler
            ):
                self.sampler = config_sampler(
                    docs=list(self.fewshot_docs()), task=self, rnd=self.fewshot_rnd
                )
            else:
                raise TypeError(
                    f"fewshot_config.sampler should be a string or callable of ContextSampler type, "
                    f"not {type(config_sampler)}"
                )

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
                self.eval_logger.error("doc_to_choice must return list")
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
                    self.eval_logger.debug(
                        f'Both target_delimiter "{self.config.target_delimiter}" and target choice: "{choice}" have '
                        f"whitespace"
                    )
                elif (not delimiter_has_whitespace) and (not choice_has_whitespace):
                    self.eval_logger.debug(
                        f'Both target_delimiter "{self.config.target_delimiter}" and target choice: "{choice}" do not '
                        f"have whitespace, ignore if the language you are evaluating on does not require/use whitespace"
                    )

    def build_all_requests(
        self,
        *,
        limit: Union[int, None] = None,
        rank: int = 0,
        world_size: int = 1,
        cache_requests: bool = False,
        rewrite_requests_cache: bool = False,
        system_instruction: Optional[str] = None,
        apply_chat_template: bool = False,
        fewshot_as_multiturn: bool = False,
        chat_template: Optional[Callable] = None,
        tokenizer_name: str = "",
    ) -> None:
        """Build a set of Instances for a task, and store them in task.instances"""

        # used with caching
        og_limit = limit

        cache_key = f"requests-{self._config.task}-{self.config.num_fewshot}shot-rank{rank}-world_size{world_size}"
        cache_key += "-chat_template" if apply_chat_template else ""
        cache_key += "-fewshot_as_multiturn" if fewshot_as_multiturn else ""
        cache_key += (
            f"-system_prompt_hash{utils.hash_string(system_instruction)}"
            if system_instruction is not None
            else ""
        )
        cache_key += f"-tokenizer{tokenizer_name}"

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

        self.eval_logger.debug(
            f"Building contexts for {self.config.task} on rank {rank}..."
        )

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
                system_instruction,
                apply_chat_template,
                fewshot_as_multiturn,
                chat_template,
            )

            # TODO: we should override self.config.repeats if doing greedy gen so users don't waste time+compute
            pocket_id = self.config.metadata["pocket_args"].doc_ids[doc_id]
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
            self.eval_logger.error(
                "Split not found in _split_ranges:",
                split=split,
                _split_ranges=_split_ranges,
            )
            raise ApplicationError(
                f"'{split}' split not found in _split_ranges: {_split_ranges.keys()}",
                non_retryable=True,
            )

    def add_range_condition(self, split: str, _split_ranges: dict):
        """
        This function constructs a BETWEEN condition for a range of ids.

        Args:
        split: The split for which the range of ids should be added (this is one of self.config.<training|validation|dev>_split)
        _split_ranges: A dictionary with the min and max ids for each split

        Returns:
        condition: A string representing a SQL BETWEEN condition
        """
        min_range = _split_ranges[split]["min"]
        max_range = _split_ranges[split]["max"]
        self.eval_logger.debug(
            "Adding range condition:",
            split=split,
            min_range=min_range,
            max_range=max_range,
        )
        condition = f"( __id BETWEEN {min_range} AND {max_range})"
        return condition

    def generate_random_doc_ids(
        self,
        table_name: str,
        _split: str,
        qty: int,
        min: int,
        max: int,
        blacklist: List[int] = [],
    ) -> List[int]:
        """
        This function generates a list of random numbers within a range, excluding some blacklisted numbers
        """
        # check that the quantity of numbers to generate is less than the range
        if qty > (max - min + 1):
            self.eval_logger.error(
                "quantity overflow:",
                table_name=table_name,
                _split=_split,
                qty=qty,
                range_min=min,
                range_max=max,
            )
            raise ApplicationError(
                "Quantity of numbers to generate is greater than the range",
                non_retryable=True,
            )

        # Generate a list of random numbers within the range [min, max]
        ints = set(range(min, max + 1))
        if blacklist is not None:
            # exclude the blacklist members
            if len(blacklist) > 0:
                original_len = len(ints)
                # Remove the blacklisted numbers
                ints = ints - set(blacklist)
                # Check that the blacklist numbers were removed
                if len(ints) == original_len:
                    self.eval_logger.error(
                        "Blacklist out of range:",
                        table_name=table_name,
                        _split=_split,
                        range_min=min,
                        range_max=max,
                        blacklist=blacklist,
                    )
                    raise ApplicationError(
                        "Blacklist corresponding to '{}' table & '{}' split were not founded in the range: [{}-{}]".format(
                            table_name, _split, min, max
                        ),
                        non_retryable=True,
                    )
        # sorted random numbers
        choices = sorted(np.random.choice(list(ints), qty, replace=False).tolist())
        self.eval_logger.debug("Random numbers generated:", choices=choices)
        return choices

    def get_all_doc_ids(self, _split: str, _split_ranges: dict) -> List[int]:
        """
        This function returns all the ids for a given split
        """
        min_range = _split_ranges[_split]["min"]
        max_range = _split_ranges[_split]["max"] + 1
        self.eval_logger.debug(
            "Getting all ids from split range:",
            split=_split,
            min_range=min_range,
            max_range=max_range,
        )
        return list(range(min_range, max_range))

    def get_SQL_where_clause(self, indexes, _split: str, _split_ranges: dict):
        """
        This function constructs a WHERE clause for a SQL query using BETWEEN for ranges.
        """

        conditions = []
        if self.config.test_split:
            self.check_split_exist(self.config.test_split, _split_ranges)
            if _split != self.config.test_split:
                self.eval_logger.error(
                    "mismatch test_split:",
                    _split=_split,
                    test_split=self.config.test_split,
                )
                raise ApplicationError(
                    f"_split '{_split}' not equal to test_split '{self.config.test_split}'",
                    non_retryable=True,
                )

            conditions.append(f"( __id IN ({', '.join(str(id) for id in indexes)}))")

            if self.config.validation_split:
                self.check_split_exist(self.config.validation_split, _split_ranges)
                conditions.append(
                    self.add_range_condition(
                        self.config.validation_split, _split_ranges
                    )
                )

            if self.config.training_split:
                self.check_split_exist(self.config.training_split, _split_ranges)
                conditions.append(
                    self.add_range_condition(self.config.training_split, _split_ranges)
                )

            if self.config.fewshot_split:
                self.check_split_exist(self.config.fewshot_split, _split_ranges)
                conditions.append(
                    self.add_range_condition(self.config.fewshot_split, _split_ranges)
                )

        elif self.config.validation_split:
            self.check_split_exist(self.config.validation_split, _split_ranges)
            if _split != self.config.validation_split:
                self.eval_logger.error(
                    "mismatch validation_split:",
                    _split=_split,
                    validation_split=self.config.validation_split,
                )
                raise ApplicationError(
                    f"_split '{_split}' not equal to validation_split '{self.config.validation_split}'",
                    non_retryable=True,
                )
            conditions.append(f"( __id IN ({', '.join(str(id) for id in indexes)}))")

            if self.config.training_split:
                self.check_split_exist(self.config.training_split, _split_ranges)
                conditions.append(
                    self.add_range_condition(self.config.training_split, _split_ranges)
                )

            if self.config.fewshot_split:
                self.check_split_exist(self.config.fewshot_split, _split_ranges)
                conditions.append(
                    self.add_range_condition(self.config.fewshot_split, _split_ranges)
                )
        else:
            self.eval_logger.error("Config without splits:", config=self.config)
            raise ApplicationError(
                "Neither test_split nor validation_split in config, cannot proceed, please check get_SQL_where_clause",
                non_retryable=True,
            )

        # This is OR because this function is used to retrieve all the samples
        # to test and all samples from the other splits. So, the result is a
        # query with the samples to tests and the other splits (probably used
        # for fewshots)
        where_clause = " OR ".join(conditions)
        return where_clause

    async def get_max_min_ids(self, postgres_conn: asyncpg.Connection, table_name: str):
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
            self.eval_logger.debug("SQL query:", sql_query=sql_query)

            # Fetch all rows from the result
            rows = await postgres_conn.fetch(sql_query)
            # assert that rows are not empty
            if len(rows) == 0:
                self.eval_logger.error(
                    "No rows found in table:",
                    table_name=table_name,
                    sql_query=sql_query,
                )
                raise ApplicationError(
                    f"No rows found in table {table_name}", non_retryable=True
                )

            _split_ranges = {}
            for row in rows:
                _split_ranges[row[0]] = {"min": row[1], "max": row[2]}
        except Exception as error:
            self.eval_logger.error("Error while connecting to PostgreSQL:", error=error)
            raise ApplicationError(
                "Error while connecting to PostgreSQL", non_retryable=True
            )

        return _split_ranges

    def get_split_from_ids(self, _split_ranges: dict, __ids: List[int]):
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
            split_ranges[k] = set(range(v["min"], v["max"] + 1))

        split_range = []
        for _id in __ids:
            for k, v in split_ranges.items():
                if _id in v:
                    split_range.append(k)
                    break
        # all ids should belong to a split range
        if len(split_range) != len(__ids):
            self.eval_logger.error(
                "Ids not in split range:", split_range=split_range, __ids=__ids
            )
            raise ApplicationError(
                "Some ids do not belong to any split range", non_retryable=True
            )

        # all ids should belong to a unique split range
        if len(set(split_range)) != 1:
            self.eval_logger.error(
                "Ids in more than one split:", __ids=__ids, split_range=split_range
            )
            raise ApplicationError(
                "Some ids belong to more than one split.", non_retryable=True
            )

        return list(set(split_range))[0]


class EvaluatePocketNetworkConfigurableTask(PocketNetworkConfigurableTask):
    async def build_all_requests(
        self,
        *,
        task_id: ObjectId,
        mongo_client: MongoClient,
        collection: str = "tasks",
        limit: Union[int, None] = None,
        rank: int = 0,
        world_size: int = 1,
        cache_requests: bool = False,
        rewrite_requests_cache: bool = False,
        system_instruction: Optional[str] = None,
        apply_chat_template: bool = False,
        fewshot_as_multiturn: bool = False,
        chat_template: Optional[Callable] = None,
        tokenizer_name: str = "",
    ) -> None:
        """Build a set of Instances for a task, and store them in task.instances"""
        (
            self._instances,
            kept_doc_ids,
            self.result_height,
            self.failed_instances,
        ) = await MongoOperator(client=mongo_client).reconstruct_instances(
            task_id=task_id, eval_logger=self.eval_logger
        )
        # Kept only those docs_ids filled by all its instances/responses
        if kept_doc_ids:
            b_dict = {}
            for i, b in enumerate(self.config.metadata["pocket_args"].doc_ids):
                b_dict[b] = i
            a_indices = [b_dict[a] for a in kept_doc_ids]
            self.dataset[self.eval_split] = Dataset.from_dict(
                self.dataset[self.eval_split][a_indices]
            )
