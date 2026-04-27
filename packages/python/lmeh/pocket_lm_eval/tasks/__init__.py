import collections
import inspect
import logging
from dataclasses import fields as dataclass_fields
from typing import Any, List, Optional, Union

import asyncpg
# from lm_eval.api.group import ConfigurableGroup  # deprecated wrapper, kept for compat
from lm_eval.config.group import GroupConfig  # moved here in TaskManager refactor
from lm_eval.tasks import TaskManager
from lm_eval.tasks._index import Kind  # new enum: TASK, PY_TASK, GROUP, TAG
from lm_eval.tasks._yaml_loader import load_yaml  # replaces utils.load_yaml_config
from temporalio.exceptions import ApplicationError

from packages.python.lmeh.pocket_lm_eval.api.task import (
    EvaluatePocketNetworkConfigurableTask,
    PocketNetworkConfigurableTask,
)
from packages.python.protocol.protocol import PocketNetworkTaskRequest

TASK_MANAGER_REGISTER_STAGE = "register"
TASK_MANAGER_SAMPLE_STAGE = "sample"
TASK_MANAGER_EVALUATE_STAGE = "evaluate"

STAGE_TYPING = Union[
    TASK_MANAGER_REGISTER_STAGE, TASK_MANAGER_SAMPLE_STAGE, TASK_MANAGER_EVALUATE_STAGE
]


class PocketNetworkTaskManager(TaskManager):
    """TaskManager subclass that loads PocketNetwork-aware task objects.

    On task loading, intercepts YAML and Python tasks to inject Pocket
    metadata and instantiate the correct Pocket task class based on the
    current workflow stage (register / sample / evaluate).

    Groups and tags are delegated entirely to the parent TaskManager /
    TaskFactory, which handles their traversal correctly.
    """

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
        # Let the new TaskManager build self._index, self._all_tasks,
        # self._all_groups, self._all_subtasks, self._all_tags.
        super().__init__(
            verbosity=verbosity,
            include_path=include_path,
            include_defaults=include_defaults,
            metadata=metadata,
        )

        # Pocket-specific state
        self.postgres_conn = postgres_conn
        self.pocket_args = pocket_args
        self.logger = logger
        self.stage = stage
        self.hf_token = hf_token
        self.task_group_map = collections.defaultdict(list)
        self.injected_metadata = {"pocket_args": self.pocket_args}

    # ------------------------------------------------------------------
    # Core override: intercept task loading at the spec level
    # ------------------------------------------------------------------

    def _load_spec(self, spec: Union[str, dict[str, Any]]) -> Any:
        """Override TaskManager._load_spec to build PocketNetwork task objects.

        Groups and tags are delegated to the parent implementation, which
        uses TaskFactory to handle traversal and child construction. Only
        leaf tasks (TASK / PY_TASK) are intercepted here.
        """
        if isinstance(spec, str):
            entry = self._index.get(spec)
            if entry is not None and entry.kind in (Kind.GROUP, Kind.TAG):
                # No Pocket-specific logic needed for groups/tags;
                # parent handles traversal and will call _load_spec
                # recursively for each child task.
                return super()._load_spec(spec)

        return self._build_pocket_task(spec)

    def _build_pocket_task(self, spec: Union[str, dict[str, Any]]) -> Any:
        """Resolve config, inject Pocket metadata, and construct a Pocket task object."""
        # ---- Resolve config dict ----------------------------------------
        if isinstance(spec, str):
            entry = self._index.get(spec)
            if entry is None:
                raise KeyError(f"Task '{spec}' not found in index")
            if entry.yaml_path is not None:
                cfg = dict(load_yaml(entry.yaml_path, resolve_func=True))
            else:
                cfg = dict(entry.cfg or {})
            cfg["task"] = spec
        else:
            # Inline dict spec (e.g. from load_task_or_group with overrides)
            cfg = dict(spec)

        # ---- Inject Pocket metadata -------------------------------------
        cfg.setdefault("metadata", {}).update(self.injected_metadata)
        if self.metadata:
            cfg["metadata"].update(self.metadata)

        # ---- Python task route (has "class" key) ------------------------
        if "class" in cfg:
            cls = cfg["class"]
            if "config" in inspect.signature(cls.__init__).parameters:
                task_object = cls(config=cfg)
            else:
                task_object = cls()
            # Mirror the old behaviour: patch task name on Pocket tasks
            if isinstance(task_object, PocketNetworkConfigurableTask):
                task_object.config.task = cfg["task"]
            return task_object

        # ---- YAML task route: choose class by stage ---------------------
        if self.stage in (TASK_MANAGER_REGISTER_STAGE, TASK_MANAGER_SAMPLE_STAGE):
            task_object = PocketNetworkConfigurableTask(
                config=cfg,
                postgres_conn=self.postgres_conn,
                eval_logger=self.logger,
                hf_token=self.hf_token,
            )
        elif self.stage == TASK_MANAGER_EVALUATE_STAGE:
            task_object = EvaluatePocketNetworkConfigurableTask(
                config=cfg,
                postgres_conn=self.postgres_conn,
                eval_logger=self.logger,
            )
        else:
            raise ApplicationError(
                f"Stage '{self.stage}' is not supported. "
                f"Must be one of: {TASK_MANAGER_REGISTER_STAGE!r}, "
                f"{TASK_MANAGER_SAMPLE_STAGE!r}, {TASK_MANAGER_EVALUATE_STAGE!r}.",
                non_retryable=True,
            )

        self.logger.debug(
            "Task successfully loaded",
            task=cfg["task"],
            task_config=task_object.config.to_dict(),
            stage=self.stage,
        )
        return task_object
