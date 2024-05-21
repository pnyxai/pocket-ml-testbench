from packages.python.common.utils import get_from_dict
from packages.python.logger.logger import get_logger
from pymongo import MongoClient
import logging
from datasets.utils import disable_progress_bars as datasets_disable_progress_bars
from evaluate.utils import disable_progress_bar as evaluate_disable_progress_bars

app_config = {
    "config": {
        "log_level": "ERROR"
    },
    # set the postgres connection here
    "postgres": None,
    # set the mongodb connection here
    "mongodb": None,
}


def setup_app(cfg) -> dict:
    """
    Setups app configuration
    :return:
    """

    app_config["config"] = cfg
    # use get_from_dict(dict, "path") or get_from_dict(dict, "nested.path") to:
    # connect mongodb
    logging.getLogger('pymongo').setLevel(get_from_dict(app_config, "config.log_level"))
    app_config["config"]["mongo_client"] = MongoClient(app_config["config"]['mongodb_uri'])
    app_config["config"]["mongo_client"].admin.command('ping')

    # disable download bars from lm_eval dependencies.
    datasets_disable_progress_bars()
    evaluate_disable_progress_bars()

    # do whatever else
    # store those shared elements on app_config
    return app_config


def get_app_config() -> dict:
    """
    Returns the global app config
    :return:
    """
    return app_config


def get_app_logger(name: str):
    return get_logger(name, get_from_dict(app_config, "config.log_level"))
