from typing import (
    Any,
    Dict,
    Iterable,
    Iterator,
    List,
    Literal,
    Mapping,
    Optional,
    Tuple,
    Union,
)
import logging
import numpy as np
import random
import datasets
from lm_eval.api import samplers
from lm_eval.api.registry import (
    AGGREGATION_REGISTRY,
    DEFAULT_METRIC_REGISTRY,
    get_aggregation,
    get_metric,
    get_metric_aggregation,
    is_higher_better,
)
from tqdm import tqdm
from lm_eval.api.task import ConfigurableTask, TaskConfig, ALL_OUTPUT_TYPES
from lm_eval.caching.cache import load_from_cache, save_to_cache
from lm_eval.filters import build_filter_ensemble
from lm_eval.prompts import get_prompt

from app.app import get_app_logger
eval_logger = get_app_logger("sample")


import psycopg2
from urllib.parse import urlparse
from temporalio.exceptions import ApplicationError

def get_max_min_ids(uri:str, table_name:str):
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
        # Parse the URI to extract connection parameters
        uri_parts = urlparse(uri)
        dbname = uri_parts.path[1:]
        username = uri_parts.username
        password = uri_parts.password
        host = uri_parts.hostname
        port = uri_parts.port

        # Connect to your PostgreSQL database
        conn = psycopg2.connect(
            dbname=dbname,
            user=username,
            password=password,
            host=host,
            port=port
        )

        # Create a cursor object using the cursor() method
        cursor = conn.cursor()

        # Construct the SQL query
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
        # Execute the SQL query
        cursor.execute(sql_query)

        # Fetch all rows from the result
        rows = cursor.fetchall()
        # assert that rows is not empty
        if len(rows) == 0:
            eval_logger.error(f"No rows found in table:", table_name=table_name, sql_query=sql_query)
            raise ApplicationError(f"No rows found in table {table_name}", non_retryable=True)

        _split_ranges = {}
        for row in rows:
            _split_ranges[row[0]] = {'min':row[1], 'max': row[2]}

    except (Exception, psycopg2.Error) as error:
        eval_logger.error(f"Error while connecting to PostgreSQL:", error=error)
        raise ApplicationError("Error while connecting to PostgreSQL", non_retryable=True)

    finally:
        # Close the cursor and database connection
        if conn:
            cursor.close()
            conn.close()

    return _split_ranges

def get_split_from_ids(_split_ranges : dict, __ids:List[int]):
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
    for k,v in _split_ranges.items():
        split_ranges[k] = set(range(v['min'],v['max']+1))

    split_range = []
    for _id in __ids:
        for k,v in split_ranges.items():
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

        eval_logger.info(f"Building contexts for {self.config.task} on rank {rank}...")

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
        max_range = _split_ranges[split]['max']+1
        eval_logger.debug(f"Adding ids from split range:", split=split, min_range=min_range, max_range=max_range)
        id_list_str += ', '.join(str(id) for id in range(min_range, max_range))
        return id_list_str
    
    def generate_random_numbers(self, table_name:str, _split:str, qty:int, min:int, max:int, blacklist: List[int] = []) -> List[int]:
        """
        This function generates a list of random numbers within a range, excluding some blacklisted numbers
        """
        # check that the quantity of numbers to generate is less than the range
        if qty > (max - min + 1):
            eval_logger.error(f"quantity overflow:", table_name=table_name, _split=_split, qty=qty, range_min=min, range_max=max)
            raise ApplicationError(
                "Quantity of numbers to generate is greater than the range", 
                non_retryable=True
                )
        # Generate a list of random numbers within the range [min, max] excluding the blacklist
        ints = set(range(min, max+1))
        if blacklist is not None:
            original_len = len(ints)
            # Remove the blacklisted numbers
            ints = ints - set(blacklist)
            # Check that the blacklist numbers were removed
            if len(ints) == original_len:
                eval_logger.error(f"Blacklist out of range:", table_name=table_name, _split=_split, range_min=min, range_max=max, blacklist=blacklist)
                raise ApplicationError(
                    "Blacklist corresponding to '{}' table & '{}' split were not founded in the range: [{}-{}]".format(table_name, _split, min, max), 
                    non_retryable=True
                    )
        # sorted random numbers
        choices = sorted(np.random.choice(list(ints), qty, replace=False).tolist())
        eval_logger.debug(f"Random numbers generated:", choices=choices)
        return choices
    
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
            
            id_list_str += ', '.join(str(id) for id in indexes)+', '

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
                eval_logger.error(f"mismatch validation_split:", _split=_split, validation_split=self.config.validation_split)
                raise ApplicationError(
                    f"_split '{_split}' not equal to validation_split '{self.config.validation_split}'",
                    non_retryable=True
                )
            id_list_str += ', '.join(str(id) for id in indexes)+', '
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
        
        where_clause = f"__id IN ({id_list_str})"

        return where_clause        
    
    def download(self, dataset_kwargs: Optional[Dict[str, Any]] = None) -> None:

        blacklist = self._config.metadata['pocket_args'].blacklist
        qty = self._config.metadata['pocket_args'].qty
        postgres_uri = self._config.metadata['pocket_args'].postgres_uri
        table_name = self.DATASET_PATH + "--" + self.DATASET_NAME if self.DATASET_NAME else self.DATASET_PATH
        eval_logger.debug(f"table_name:",table_name=table_name)
        _split_ranges = get_max_min_ids(table_name=table_name, uri=postgres_uri)
        eval_logger.debug(f"Split ranges:",_split_ranges=_split_ranges)

        # Its necesarry to detect wich is the split used to test to take the range, and then get random indexes
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
        indexes = self.generate_random_numbers(table_name, _split, qty, _range['min'], _range['max'], blacklist)

        where_clause = self.get_SQL_where_clause(indexes, _split, _split_ranges)
        # Construct the full SQL query
        sql_query = f"SELECT * FROM \"{table_name}\" WHERE {where_clause};"
        ds = datasets.Dataset.from_sql(sql_query, con = postgres_uri)
        dataset = datasets.DatasetDict()
        for split in ds.unique("__split"):
            eval_logger.debug(f"Adding split to DatasetDict:", split=split)
            dataset[split] = ds.filter(lambda x: x["__split"] == split)
        self.dataset = dataset.remove_columns(["__id", "__split"])
        # save in config the indexes used to download the dataset
        self._config.metadata['pocket_args'].doc_ids = indexes
