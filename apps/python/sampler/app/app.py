from packages.python.common.utils import get_from_dict
from packages.python.logger.logger import get_logger
from pymongo import MongoClient

app_config = {
    "config": {
        "log_level": "DEBUG"
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
    # connect postgres
    # connect mongodb
    try:
        app_config["config"]["mongo_client"] = MongoClient(app_config["config"]['mongodb_uri'])
        app_config["config"]["mongo_client"].admin.command('ping')
    except Exception as e:
        raise e
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
