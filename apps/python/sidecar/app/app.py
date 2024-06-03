import logging

from transformers.utils.logging import set_verbosity as trasformer_set_verbosity

from packages.python.common.utils import get_from_dict
from packages.python.logger.logger import get_logger

app_config = {
    "config": {"log_level": "ERROR"},
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

    # set transformers verbosity.
    log_level = get_from_dict(app_config, "config.log_level")
    trasformer_set_verbosity(logging.getLevelName(log_level))

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
