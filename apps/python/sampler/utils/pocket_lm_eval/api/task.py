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

# class PocketNetworkTaskConfig(TaskConfig):
#     pocket_args: Optional[Dict[str, Any]] = None

class PocketNetworkConfigurableTask(ConfigurableTask):
    # VERSION = "Yaml"
    # OUTPUT_TYPE = None
    # CONFIG = None

    # def __init__(
    #     self,
    #     data_dir=None,
    #     cache_dir=None,
    #     download_mode=None,
    #     config: Optional[dict] = None,
    # ) -> None:  # TODO no super() call here
    #     # Get pre-configured attributes
    #     self._config = self.CONFIG

    #     # Use new configurations if there was no preconfiguration
    #     if self.config is None:
    #         self._config = PocketNetworkTaskConfig(**config)
    #     # Overwrite configs
    #     else:
    #         if config is not None:
    #             self._config.__dict__.update(config)

    #     if self.config is None:
    #         raise ValueError(
    #             "Must pass a config to ConfigurableTask, either in cls.CONFIG or `config` kwarg"
    #         )

    #     if isinstance(self.config.metadata, dict):
    #         if "version" in self.config.metadata:
    #             self.VERSION = self.config.metadata["version"]

    #     if self.config.output_type is not None:
    #         if self.config.output_type not in ALL_OUTPUT_TYPES:
    #             raise ValueError(
    #                 f"Got invalid output_type '{self.config.output_type}', must be in '{','.join(ALL_OUTPUT_TYPES)}'"
    #             )
    #         self.OUTPUT_TYPE = self.config.output_type

    #     if self.config.dataset_path is not None:
    #         self.DATASET_PATH = self.config.dataset_path

    #     if self.config.dataset_name is not None:
    #         self.DATASET_NAME = self.config.dataset_name

    #     self._metric_fn_list = {}
    #     self._metric_fn_kwargs = {}
    #     self._aggregation_list = {}
    #     self._higher_is_better = {}

    #     if self.config.metric_list is None:
    #         # TODO: handle this in TaskConfig.__post_init__ ?
    #         _metric_list = DEFAULT_METRIC_REGISTRY[self.config.output_type]

    #         for metric_name in _metric_list:
    #             self._metric_fn_list[metric_name] = get_metric(metric_name)
    #             self._metric_fn_kwargs[metric_name] = {}
    #             self._aggregation_list[metric_name] = get_metric_aggregation(
    #                 metric_name
    #             )
    #             self._higher_is_better[metric_name] = is_higher_better(metric_name)
    #     else:
    #         for metric_config in self.config.metric_list:
    #             if "metric" not in metric_config:
    #                 raise ValueError(
    #                     "'metric' key not provided for an entry in 'metric_list', must be specified!"
    #                 )
    #             metric_name = metric_config["metric"]
    #             kwargs = {
    #                 key: metric_config[key]
    #                 for key in metric_config
    #                 if key
    #                 not in ["metric", "aggregation", "higher_is_better", "hf_evaluate"]
    #             }
    #             hf_evaluate_metric = (
    #                 "hf_evaluate" in metric_config
    #                 and metric_config["hf_evaluate"] is True
    #             )

    #             if self.config.process_results is not None:
    #                 self._metric_fn_list[metric_name] = None
    #                 self._metric_fn_kwargs[metric_name] = {}
    #             elif callable(metric_name):
    #                 metric_fn = metric_name.__call__
    #                 metric_name = metric_name.__name__
    #                 self._metric_fn_list[metric_name] = metric_fn
    #                 self._metric_fn_kwargs[metric_name] = kwargs
    #             else:
    #                 self._metric_fn_list[metric_name] = get_metric(
    #                     metric_name, hf_evaluate_metric
    #                 )
    #                 self._metric_fn_kwargs[metric_name] = kwargs

    #             if "aggregation" in metric_config:
    #                 agg_name = metric_config["aggregation"]
    #                 if isinstance(agg_name, str):
    #                     self._aggregation_list[metric_name] = get_aggregation(agg_name)
    #                 elif callable(agg_name):  # noqa: E721
    #                     self._aggregation_list[metric_name] = metric_config[
    #                         "aggregation"
    #                     ]
    #             else:
    #                 INV_AGG_REGISTRY = {v: k for k, v in AGGREGATION_REGISTRY.items()}
    #                 metric_agg = get_metric_aggregation(metric_name)
    #                 eval_logger.warning(
    #                     f"[Task: {self.config.task}] metric {metric_name} is defined, but aggregation is not. "
    #                     f"using default "
    #                     f"aggregation={INV_AGG_REGISTRY[metric_agg]}"
    #                 )
    #                 self._aggregation_list[metric_name] = metric_agg

    #             if "higher_is_better" in metric_config:
    #                 self._higher_is_better[metric_name] = metric_config[
    #                     "higher_is_better"
    #                 ]
    #             else:
    #                 eval_logger.warning(
    #                     f"[Task: {self.config.task}] metric {metric_name} is defined, but higher_is_better is not. "
    #                     f"using default "
    #                     f"higher_is_better={is_higher_better(metric_name)}"
    #                 )
    #                 self._higher_is_better[metric_name] = is_higher_better(metric_name)

    #     self.download(self.config.dataset_kwargs)
    #     self._training_docs = None
    #     self._fewshot_docs = None

    #     if self.config.filter_list is not None:
    #         self._filters = []
    #         for filter_config in self.config.filter_list:
    #             filter_name = filter_config["name"]
    #             filter_functions = filter_config["filter"]
    #             components = []
    #             for function in filter_functions:
    #                 kwargs = {
    #                     key: function[key] for key in function if key != "function"
    #                 }
    #                 components.append([function["function"], kwargs])
    #             filter_pipeline = build_filter_ensemble(filter_name, components)
    #             self._filters.append(filter_pipeline)
    #     else:
    #         self._filters = [build_filter_ensemble("none", [["take_first", None]])]

    #     if self.config.use_prompt is not None:
    #         eval_logger.info(f"loading prompt {self.config.use_prompt}")
    #         self.prompt = get_prompt(
    #             self.config.use_prompt, self.DATASET_PATH, self.DATASET_NAME
    #         )
    #     else:
    #         self.prompt = None

    #     if self.fewshot_docs() is not None:
    #         self.sampler = samplers.get_sampler(
    #             self.config.fewshot_config.get("sampler", "default")
    #             if self.config.fewshot_config
    #             else "default"
    #         )(list(self.fewshot_docs()), self, rnd=random.Random(1234))

    #     self.task_docs = self.eval_docs

    #     # Test One Doc
    #     self.features = list(self.task_docs.features.keys())
    #     self.multiple_input = 0
    #     self.multiple_target = 0
    #     test_doc = self.task_docs[0]
    #     test_text = self.doc_to_text(test_doc)
    #     test_target = self.doc_to_target(test_doc)

    #     if self.config.doc_to_choice is not None:
    #         test_choice = self.doc_to_choice(test_doc)
    #         if not isinstance(test_choice, list):
    #             eval_logger.error("doc_to_choice must return list")
    #         else:
    #             num_choice = len(test_choice)

    #         if isinstance(test_text, int):
    #             self.multiple_input = num_choice
    #     else:
    #         test_choice = None

    #     if isinstance(test_target, list):
    #         self.multiple_target = len(test_target)
    #     else:
    #         if (isinstance(test_target, int)) and (test_choice is not None):
    #             test_target = test_choice[test_target]
    #         else:
    #             test_target = str(test_target)

    #     if test_choice is not None:
    #         check_choices = test_choice
    #     else:
    #         check_choices = [test_target]
    #     if self.config.doc_to_choice is not None:
    #         for choice in check_choices:
    #             choice_has_whitespace = True if choice[0].isspace() else False
    #             delimiter_has_whitespace = (
    #                 True
    #                 if self.config.target_delimiter.rstrip()
    #                 != self.config.target_delimiter
    #                 else False
    #             )

    #             if delimiter_has_whitespace and choice_has_whitespace:
    #                 eval_logger.debug(
    #                     f'Both target_delimiter "{self.config.target_delimiter}" and target choice: "{choice}" have whitespace'
    #                 )
    #             elif (not delimiter_has_whitespace) and (not choice_has_whitespace):
    #                 eval_logger.debug(
    #                     f'Both target_delimiter "{self.config.target_delimiter}" and target choice: "{choice}" do not have whitespace, ignore if the language you are evaluating on does not require/use whitespace'
    #                 )

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