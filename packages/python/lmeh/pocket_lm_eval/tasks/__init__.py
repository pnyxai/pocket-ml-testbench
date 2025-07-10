import collections
import logging
from functools import partial
from typing import List, Mapping, Optional, Union


import asyncpg
from lm_eval import utils
from lm_eval.api.group import ConfigurableGroup, GroupConfig
from lm_eval.tasks import TaskManager
from temporalio.exceptions import ApplicationError

from packages.python.lmeh.pocket_lm_eval.api.task import (
    EvaluatePocketNetworkConfigurableTask,
    PocketNetworkConfigurableTask,
)
from packages.python.protocol.protocol import PocketNetworkTaskRequest


GROUP_ONLY_KEYS = list(GroupConfig().to_dict().keys())
TASK_MANAGER_REGISTER_STAGE = "register"
TASK_MANAGER_SAMPLE_STAGE = "sample"
TASK_MANAGER_EVALUATE_STAGE = "evaluate"

STAGE_TYPING = Union[
    TASK_MANAGER_REGISTER_STAGE, TASK_MANAGER_SAMPLE_STAGE, TASK_MANAGER_EVALUATE_STAGE
]


class PocketNetworkTaskManager(TaskManager):
    def __init__(
        self,
        postgres_conn: asyncpg.Connection,
        stage: STAGE_TYPING,
        verbosity="ERROR",
        include_path: Optional[Union[str, List]] = None,
        include_defaults: bool = True,
        metadata: Optional[dict] = None,
        pocket_args: PocketNetworkTaskRequest = None,
        logger: Optional[logging.Logger] = None,
        hf_token: Optional[str] = None,
    ) -> None:
        self.verbosity = verbosity
        self.include_path = include_path
        self.metadata = metadata
        self.pocket_args = pocket_args
        self.postgres_conn = postgres_conn
        self.logger = logger
        self._task_index = self.initialize_tasks(
            include_path=include_path, include_defaults=include_defaults
        )
        self._all_tasks = sorted(list(self._task_index.keys()))

        self._all_groups = sorted(
            [x for x in self._all_tasks if self._task_index[x]["type"] == "group"]
        )
        self._all_subtasks = sorted(
            [
                x
                for x in self._all_tasks
                if self._task_index[x]["type"] in ["task", "python_task"]
            ]
        )
        self._all_tags = sorted(
            [x for x in self._all_tasks if self._task_index[x]["type"] == "tag"]
        )
        self.stage = stage

        self.task_group_map = collections.defaultdict(list)
        self.injected_metadata = {
            "pocket_args": self.pocket_args,
        }
        self.hf_token = hf_token

    """PocketNetworkTaskManager indexes all tasks from the default `lm_eval/tasks/`
    and an optional directory if provided.

    """

    def _load_individual_task_or_group(
        self,
        name_or_config: Optional[Union[str, dict]] = None,
        parent_name: Optional[str] = None,
        update_config: Optional[dict] = None,
    ) -> Mapping:
        def _load_task(config, task):
            if "include" in config:
                config = {
                    **utils.load_yaml_config(
                        yaml_path=None,
                        yaml_config={"include": config.pop("include")},
                        mode="full",
                    ),
                    **config,
                }
            if self._config_is_python_task(config):
                if self._class_has_config_in_constructor(config["class"]):
                    task_object = config["class"](config=config)
                else:
                    task_object = config["class"]()
                if isinstance(task_object, PocketNetworkConfigurableTask):
                    # very scuffed: set task name here. TODO: fixme?
                    task_object.config.task = task
            else:
                if self.metadata is not None:
                    config["metadata"] = config.get("metadata", {}) | self.metadata
                else:
                    config["metadata"] = config.get("metadata", {})

                if (
                    self.stage == TASK_MANAGER_REGISTER_STAGE
                    or self.stage == TASK_MANAGER_SAMPLE_STAGE
                ):
                    task_object = PocketNetworkConfigurableTask(
                        config=config,
                        postgres_conn=self.postgres_conn,
                        eval_logger=self.logger,
                        hf_token=self.hf_token,
                    )
                elif self.stage == TASK_MANAGER_EVALUATE_STAGE:
                    task_object = EvaluatePocketNetworkConfigurableTask(
                        config=config,
                        postgres_conn=self.postgres_conn,
                        eval_logger=self.logger,
                    )
                else:
                    ApplicationError(
                        f"Stage {self.stage} not supported", non_retryable=True
                    )

            self.logger.debug(
                "Task successfully loaded",
                task=task,
                task_config=task_object.config.to_dict(),
                stage=self.stage,
            )
            return {task: task_object}

        def _get_group_and_subtask_from_config(
            config: dict,
        ) -> tuple[ConfigurableGroup, list[str]]:
            if self.metadata is not None:
                config["metadata"] = config.get("metadata", {}) | self.metadata
            group_name = ConfigurableGroup(config=config)
            subtask_list = []
            for task in group_name.config["task"]:
                if isinstance(task, str) and self._name_is_tag(task):
                    subtask_list.extend(self._get_tasklist(task))
                else:
                    subtask_list.append(task)
            return group_name, subtask_list

        def _process_group_config(
            config: dict, update_config: dict = None
        ) -> tuple[dict, dict]:
            if update_config is not None:
                config = {**config, **update_config}
            _update_config = {
                k: v for k, v in config.items() if k not in GROUP_ONLY_KEYS
            }
            if not bool(_update_config):
                _update_config = None

            group_config = {k: v for k, v in config.items() if k in GROUP_ONLY_KEYS}
            return group_config, _update_config

        if isinstance(name_or_config, str):
            if update_config is not None:
                # Process name_or_config as a dict instead
                name_or_config = {"task": name_or_config, **update_config}
            elif self._name_is_task(name_or_config) or self._name_is_python_task(
                name_or_config
            ):
                task_config = self._get_config(name_or_config)
                ############################################################
                # START: POCKET NETWORK CODE
                ############################################################
                if "metadata" in task_config.keys():
                    task_config["metadata"].update(self.injected_metadata)
                else:
                    task_config["metadata"] = self.injected_metadata
                ############################################################
                # END: POCKET NETWORK CODE
                ############################################################
                return _load_task(task_config, task=name_or_config)
            else:
                subtask_list = self._get_tasklist(name_or_config)
                if subtask_list == -1:
                    group_config = self._get_config(name_or_config)
                    group_config, update_config = _process_group_config(group_config)
                    group_name, subtask_list = _get_group_and_subtask_from_config(
                        group_config
                    )
                else:
                    if self._name_is_tag(name_or_config):
                        fn = partial(
                            self._load_individual_task_or_group,
                            update_config=name_or_config
                            if isinstance(name_or_config, dict)
                            else None,
                        )
                        return dict(
                            collections.ChainMap(*map(fn, reversed(subtask_list)))
                        )
                    else:
                        group_name = ConfigurableGroup(
                            config={"group": name_or_config, "task": subtask_list}
                        )

        if isinstance(name_or_config, dict):
            if self._config_is_task(name_or_config):
                name = name_or_config.pop("task")
                if update_config is not None:
                    name_or_config = {**name_or_config, **update_config}

                # If the name is registered as a group
                if self._name_is_group(name):
                    group_config = self._get_config(name)

                    group_config, update_config = _process_group_config(
                        group_config, name_or_config
                    )
                    group_name, subtask_list = _get_group_and_subtask_from_config(
                        group_config
                    )
                elif self._name_is_tag(name):
                    subtask_list = self._get_tasklist(name)
                    fn = partial(
                        self._load_individual_task_or_group,
                        update_config=name_or_config,
                    )
                    return dict(collections.ChainMap(*map(fn, reversed(subtask_list))))
                else:
                    if self._name_is_registered(name):
                        base_task_config = self._get_config(name)

                        # Check if this is a duplicate.
                        if parent_name is not None:
                            num_duplicate = len(
                                list(
                                    filter(
                                        lambda x: x.startswith(name),
                                        self.task_group_map[parent_name],
                                    )
                                )
                            )
                            if num_duplicate > 0:
                                name = f"{name}-{num_duplicate}"
                            self.task_group_map[parent_name].append(name)

                        task_config = {
                            **base_task_config,
                            **name_or_config,
                        }
                    else:
                        task_config = name_or_config
                        ############################################################
                        # START: POCKET NETWORK CODE
                        ############################################################
                        if "metadata" in task_config.keys():
                            task_config["metadata"].update(self.injected_metadata)
                        else:
                            task_config["metadata"] = self.injected_metadata
                        ############################################################
                        # END: POCKET NETWORK CODE
                        ############################################################
                    return _load_task(task_config, task=name)
            else:
                group_config, update_config = _process_group_config(name_or_config)
                group_name, subtask_list = _get_group_and_subtask_from_config(
                    group_config
                )

        fn = partial(
            self._load_individual_task_or_group,
            parent_name=group_name,
            update_config=update_config,
        )
        return {
            group_name: dict(collections.ChainMap(*map(fn, reversed(subtask_list))))
        }
