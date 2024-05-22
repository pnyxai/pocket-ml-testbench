import logging
from urllib.parse import urlparse
import psycopg2
from pymongo import MongoClient
from packages.python.common.utils import get_from_dict
from packages.python.logger.logger import get_logger
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

    # create postgres connection
    uri_parts = urlparse(get_from_dict(app_config, "config.postgres_uri"))
    dbname = uri_parts.path[1:]
    username = uri_parts.username
    password = uri_parts.password
    host = uri_parts.hostname
    port = uri_parts.port
    conn = psycopg2.connect(
        dbname=dbname,
        user=username,
        password=password,
        host=host,
        port=port
    )

    cur = None
    try:
        # create a new cursor
        cur = conn.cursor()
        # execute a simple query
        cur.execute('SELECT 1')
    except Exception:
        # if it fails, print an error message
        raise Exception("The connection is no longer live")
    finally:
        # close the cursor
        if cur:
            cur.close()

    app_config["postgres"] = conn

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
