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
from lm_eval.api.task import ConfigurableTask, TaskConfig, ALL_OUTPUT_TYPES
from lm_eval.filters import build_filter_ensemble
from lm_eval.prompts import get_prompt

eval_logger = logging.getLogger("lm-eval")

import psycopg2
from urllib.parse import urlparse

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
                {}
            WHERE
                "__split" IN ('train', 'validation', 'test')
            GROUP BY
                "__split";
        """.format(table_name)

        # Execute the SQL query
        cursor.execute(sql_query)

        # Fetch all rows from the result
        rows = cursor.fetchall()
        _split_ranges = {}
        # Print the result
        for row in rows:
            _split_ranges[row[0]] = {'min':row[1], 'max': row[2]}

    except (Exception, psycopg2.Error) as error:
        print("Error while connecting to PostgreSQL", error)

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
        raise ValueError("Some ids do not belong to any split range")

    # all ids should belong to a unique split range
    if len(set(split_range)) != 1:
        raise ValueError("Some ids belong to more than one split. Please check that ids belong to only one split (test or validation).")


    return list(set(split_range))[0]




class PocketNetworkConfigurableTask(ConfigurableTask):

    def check_split_exist(self, split: str, _split_ranges: dict):
        """
        This function checks if a self.config.split exists in the keys of _split_ranges
        """
        if split not in _split_ranges.keys():
            raise ValueError(
                f"'{split}' split not found in _split_ranges: {_split_ranges.keys()}"
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
        eval_logger.debug(f"adding split \'{split}\' to id_list_str from ranges: {min_range}-{max_range}")
        id_list_str += ', '.join(str(id) for id in range(min_range, max_range))
        return id_list_str
    
    def generate_random_numbers(self, table_name:str, _split:str, qty:int, min:int, max:int, blacklist: List[int] = []) -> List[int]:
        """
        This function generates a list of random numbers within a range, excluding some blacklisted numbers
        """
        # check that the quantity of numbers to generate is less than the range
        if qty > (max - min + 1):
            raise ValueError("Quantity of numbers to generate is greater than the range")
        # Generate a list of random numbers within the range [min, max] excluding the blacklist
        ints = set(range(min, max+1))
        if blacklist is not None:
            original_len = len(ints)
            # Remove the blacklisted numbers
            ints = ints - set(blacklist)
            # Check that the blacklist numbers were removed
            if len(ints) == original_len:
                raise ValueError("Blacklist numbers corresponding to '{}' table & '{}' split were not founded in the range [min, max] generated: [{}-{}]".format(table_name, _split, min, max))

        choices = list(np.random.choice(list(ints), qty, replace=False))

        return choices
    
    def get_SQL_where_clause(self, indexes, _split: str, _split_ranges: dict):
        """
        This function constructs a WHERE clause for a SQL query. Apply the logic detailed in 
        
        """

        id_list_str = ''
        if self.config.test_split:
            eval_logger.debug("in self.config.test_split")
            self.check_split_exist(self.config.test_split, _split_ranges)
            if _split != self.config.test_split:
                raise ValueError(
                    f"_split '{_split}' not equal to test_split '{self.config.test_split}'"
                )
            
            id_list_str += ', '.join(str(id) for id in indexes)+', '

            eval_logger.debug(f"Test split:\n {id_list_str}")
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
            eval_logger.debug(f"in self.config.validation_split")            
            self.check_split_exist(self.config.validation_split, _split_ranges)
            if _split != self.config.validation_split:
                raise ValueError(
                    f"_split '{_split}' not equal to validation_split '{self.config.validation_split}'"
                )
            id_list_str += ', '.join(str(id) for id in indexes)+', '
            eval_logger.debug(f"Validation split:\n {id_list_str}")
            if self.config.training_split:
                self.check_split_exist(self.config.training_split, _split_ranges)
                id_list_str = self.add_string_ids_range(self.config.training_split, id_list_str, _split_ranges)

            if self.config.fewshot_split:
                self.check_split_exist(self.config.fewshot_split, _split_ranges)
                id_list_str = self.add_string_ids_range(self.config.fewshot_split, id_list_str, _split_ranges)
        else:
            # error
            raise ValueError("Neither test_split nor validation_split in config, cannot proceed, please check get_SQL_where_clause")
        
        where_clause = f"__id IN ({id_list_str})"

        return where_clause        
    
    def download(self, dataset_kwargs: Optional[Dict[str, Any]] = None) -> None:

        blacklist = self._config.metadata['pocket_args']['blacklist']
        qty = self._config.metadata['pocket_args']['qty']
        uri = self._config.metadata['pocket_args']['uri']
        table_name = self.DATASET_PATH + "--" + self.DATASET_NAME if self.DATASET_NAME else self.DATASET_PATH
        _split_ranges = get_max_min_ids(table_name=table_name, uri=uri)
        eval_logger.debug(f"Split ranges:\n{ _split_ranges}")

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
            raise ValueError(f"Neither {self.config.test_split} nor {self.config.validation_split} in splits were found in '_split_ranges'. Available splits are {_split_ranges.keys()}")

        _range = _split_ranges[_split]
        indexes = self.generate_random_numbers(table_name, _split, qty, _range['min'], _range['max'], blacklist)

        where_clause = self.get_SQL_where_clause(indexes, _split, _split_ranges)
        # Construct the full SQL query
        sql_query = f"SELECT * FROM {table_name} WHERE {where_clause};"
    
        ds = datasets.Dataset.from_sql(sql_query, con = uri)
        dataset = datasets.DatasetDict()
        for split in ds.unique("__split"):
            eval_logger.debug(f"Split: {split}")
            dataset[split] = ds.filter(lambda x: x["__split"] == split)
        self.dataset = dataset.remove_columns(["__id", "__split"])
