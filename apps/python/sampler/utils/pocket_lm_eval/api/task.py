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

class PocketNetworkConfigurableTask(ConfigurableTask):

    def download(self, dataset_kwargs: Optional[Dict[str, Any]] = None) -> None:
        
        indexes = self._config.metadata['pocket_args']['__id']
        uri = self._config.metadata['pocket_args']['uri']

        # Construct the WHERE clause with an IN condition
        id_list_str = ', '.join(str(id) for id in indexes)
        where_clause = f"__id IN ({id_list_str})"
        # Construct the full SQL query
        table_name = self.DATASET_PATH + "--" + self.DATASET_NAME if self.DATASET_NAME else self.DATASET_PATH
        sql_query = f"SELECT * FROM {table_name} WHERE {where_clause};"
        ds = datasets.Dataset.from_sql(sql_query, con = uri)
        dataset = datasets.DatasetDict()
        for split in ds.unique("__split"):
            dataset[split] = ds.filter(lambda x: x["__split"] == split)
        self.dataset = dataset.remove_columns(["__id", "__split"])