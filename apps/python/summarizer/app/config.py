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
        "task_queue": "summarize",
    },
}


def read_config():
    # default one
    logger = get_logger("config")
    config_path = os.getenv("CONFIG_PATH")
    if not config_path:
        logger.error("CONFIG_PATH not set.")
        return {}

    try:
        with open(config_path, "r") as f:
            config = json.load(f)
            config_with_default = deep_update(default_config, config)
            return config_with_default
    except FileNotFoundError:
        logger.error(f"file not found at {config_path}")
        return {}
