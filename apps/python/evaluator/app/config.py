import json
import os
import logging

from packages.python.common.utils import deep_update
from packages.python.logger.logger import get_logger

# Default configs that will replace/fill the missing one on the CONFIG_PATH provided
default_config = {
    "postgres_uri": "postgresql://localhost:5432",
    "mongodb_uri": "mongodb://localhost:27017",
    "log_level": logging.getLevelName(logging.ERROR),
    "temporal": {
        "host": "localhost",
        "port": 7233,
        "namespace": "pocket-ml-testbench",
        "task_queue": "sampler",
        "max_workers": 20,
        "max_cached_workflows": 2000,
        "max_concurrent_workflow_tasks": 2000,
        "max_concurrent_activities": 100000,
        "max_concurrent_workflow_task_polls": 10,
        "nonsticky_to_sticky_poll_ratio": 0.5,
        "max_concurrent_activity_task_polls": 50,
        "max_activities_per_second": 10,
        "max_task_queue_activities_per_second": 10,
    }
}


def read_config():
    # default one
    logger = get_logger("config")
    config_path = os.getenv("CONFIG_PATH")
    if not config_path:
        logger.error("CONFIG_PATH not set.")
        return {}

    try:
        with open(config_path, 'r') as f:
            config = json.load(f)
            config_with_default = deep_update(default_config, config)
            return config_with_default
    except FileNotFoundError:
        logger.error(f"file not found at {config_path}")
        return {}
