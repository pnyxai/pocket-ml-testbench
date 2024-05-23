import logging
import asyncpg
from pymongo import MongoClient
from packages.python.common.utils import get_from_dict
from packages.python.logger.logger import get_logger
from datasets.utils import disable_progress_bars as datasets_disable_progress_bars
from evaluate.utils import disable_progress_bar as evaluate_disable_progress_bars
from transformers.utils.logging import set_verbosity as trasformer_set_verbosity

app_config = {
    "config": {
        "log_level": "ERROR"
    },
    # set the postgres connection here
    "postgres": None,
    # set the mongodb connection here
    "mongodb": None,
}


async def setup_app(cfg) -> dict:
    """
    Setups app configuration
    :return:
    """

    app_config["config"] = cfg
    # use get_from_dict(dict, "path") or get_from_dict(dict, "nested.path") to:
    # connect mongodb
    log_level = get_from_dict(app_config, "config.log_level")
    logging.getLogger('pymongo').setLevel(log_level)
    app_config["config"]["mongo_client"] = MongoClient(app_config["config"]['mongodb_uri'])
    app_config["config"]["mongo_client"].admin.command('ping')

    # create postgres connection
    max_workers = get_from_dict(app_config, "config.temporal.max_workers")

    pg_pool = await asyncpg.create_pool(
        dsn=get_from_dict(app_config, "config.postgres_uri"),
        min_size=max_workers,
        max_size=max_workers,
    )

    async with pg_pool.acquire() as conn:
        await conn.execute('SELECT 1')

    app_config["postgres"] = pg_pool
    
    # disable download bars from lm_eval dependencies.
    trasformer_set_verbosity(logging.getLevelName(log_level))
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
