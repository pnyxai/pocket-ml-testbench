import collections
import logging
import asyncpg
from functools import partial
from typing import Mapping, Optional, Union
from temporalio.exceptions import ApplicationError
from lm_eval import utils
from lm_eval.tasks import TaskManager

from packages.python.lmeh.pocket_lm_eval.api.task import PocketNetworkConfigurableTask, \
    EvaluatePocketNetworkConfigurableTask

from packages.python.protocol.protocol import PocketNetworkTaskRequest

TASK_MANAGER_REGISTER_STAGE = "register"
TASK_MANAGER_SAMPLE_STAGE = "register"
TASK_MANAGER_EVALUATE_STAGE = "register"

STAGE_TYPING = Union[TASK_MANAGER_REGISTER_STAGE, TASK_MANAGER_SAMPLE_STAGE, TASK_MANAGER_EVALUATE_STAGE]


class PocketNetworkTaskManager(TaskManager):
    def __init__(
            self,
            postgres_conn: asyncpg.Connection,
            stage: STAGE_TYPING,
            verbosity="ERROR",
            include_path: Optional[str] = None,
            pocket_args: PocketNetworkTaskRequest = None,
            logger: Optional[logging.Logger] = None,
    ) -> None:
        self.verbosity = verbosity
        self.include_path = include_path
        self.pocket_args = pocket_args
        self.postgres_conn = postgres_conn
        self.logger = logger
        self._task_index = self.initialize_tasks(include_path=include_path)
        self._all_tasks = sorted(list(self._task_index.keys()))
        self.stage = stage

        self.task_group_map = collections.defaultdict(list)
        self.injected_metadata = {
            'pocket_args': self.pocket_args,
        }

    """PocketNetworkTaskManager indexes all tasks from the default `lm_eval/tasks/`
    and an optional directory if provided.

    """

    def _load_individual_task_or_group(
            self,
            name_or_config: Optional[Union[str, dict]] = None,
            parent_name: Optional[str] = None,
            update_config: Optional[dict] = None,
            yaml_path: Optional[str] = None,
    ) -> Mapping:
        def load_task(config, task, group=None, yaml_path=None):
            if "include" in config:
                if yaml_path is None:
                    raise ApplicationError(f"YAML path not provided for {task}", non_retryable=True)
                config.update(
                    utils.load_yaml_config(
                        yaml_path,
                        yaml_config={"include": config.pop("include")},
                        mode="full",
                    )
                )
            if self._config_is_python_task(config):
                task_object = config["class"]()
            else:
                config = self._process_alias(config, group=group)
                if self.stage == TASK_MANAGER_REGISTER_STAGE or self.stage == TASK_MANAGER_SAMPLE_STAGE:
                    task_object = PocketNetworkConfigurableTask(config=config, postgres_conn=self.postgres_conn)
                elif self.stage == TASK_MANAGER_EVALUATE_STAGE:
                    task_object = EvaluatePocketNetworkConfigurableTask(config=config, postgres_conn=self.postgres_conn)
                else:
                    ApplicationError(f"Stage {self.stage} not supported", non_retryable=True)
            if group is not None:
                task_object = (group, task_object)
            return {task: task_object}

        if isinstance(name_or_config, str):
            if update_config is not None:
                # Process name_or_config as a dict instead
                name_or_config = {"task": name_or_config, **update_config}
            elif self._name_is_task(name_or_config):
                task_config = self._get_config(name_or_config)
                ############################################################
                # START: POCKET NETWORK CODE
                ############################################################
                if 'metadata' in task_config.keys():
                    task_config['metadata'].update(self.injected_metadata)
                else:
                    task_config['metadata'] = self.injected_metadata
                ############################################################
                # END: POCKET NETWORK CODE
                ############################################################                    
                return load_task(task_config, task=name_or_config, group=parent_name)
            else:
                group_name = name_or_config
                subtask_list = self._get_tasklist(name_or_config)
                if subtask_list == -1:
                    group_config = self._get_config(name_or_config)
                    subtask_list = group_config["task"]

                # This checks if we're at the root.
                if parent_name is None:
                    group_config = self._get_config(name_or_config)
                    if set(group_config.keys()) > {"task", "group"}:
                        update_config = {
                            k: v
                            for k, v in group_config.items()
                            if k not in ["task", "group"]
                        }
                    yaml_path = self._get_yaml_path(group_name)

                    if (update_config is not None) and ("group_alias" in update_config):
                        group_name = update_config["group_alias"]
                        update_config.pop("group_alias")

        if isinstance(name_or_config, dict):
            if update_config is not None:
                name_or_config = {
                    **name_or_config,
                    **update_config,
                }

            if self._config_is_task(name_or_config):
                name = name_or_config["task"]
                # If the name is registered as a group
                # if self._name_is_task(name) is False:
                if self._name_is_group(name):
                    group_name = name
                    update_config = {
                        k: v for k, v in name_or_config.items() if k != "task"
                    }
                    subtask_list = self._get_tasklist(name)
                    if subtask_list == -1:
                        subtask_list = self._get_config(name)["task"]
                else:
                    if self._name_is_registered(name):
                        base_task_config = self._get_config(name)

                        # Check if this is a duplicate.
                        if parent_name is not None:
                            name_or_config["group"] = parent_name
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
                        if 'metadata' in task_config.keys():
                            task_config['metadata'].update(self.injected_metadata)
                        else:
                            task_config['metadata'] = self.injected_metadata
                        ############################################################
                        # END: POCKET NETWORK CODE
                        ############################################################
                    return load_task(
                        task_config, task=name, group=parent_name, yaml_path=yaml_path
                    )
            else:
                group_name = name_or_config["group"]
                subtask_list = name_or_config["task"]
                if set(name_or_config.keys()) > {"task", "group"}:
                    update_config = {
                        k: v
                        for k, v in name_or_config.items()
                        if k not in ["task", "group"]
                    }

        all_subtasks = {}
        if parent_name is not None:
            all_subtasks = {group_name: (parent_name, None)}

        fn = partial(
            self._load_individual_task_or_group,
            parent_name=group_name,
            update_config=update_config,
            yaml_path=yaml_path,
        )
        all_subtasks = {
            **all_subtasks,
            **dict(collections.ChainMap(*map(fn, subtask_list))),
        }
        return all_subtasks
