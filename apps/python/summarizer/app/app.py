import logging
import asyncpg
from packages.python.common.mongodb import MongoClient
from packages.python.common.utils import get_from_dict
from packages.python.logger.logger import get_logger
from packages.python.taxonomies.utils import load_taxonomy

import os

app_config = {
    "config": {"log_level": "ERROR"},
    # set the postgres connection here
    "postgres": None,
    # set the mongodb connection here
    "mongodb": None,
    # fill with taxonomies graphs here
    "taxonomies": None,
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
    logging.getLogger("motor").setLevel(log_level)
    mongo_client = MongoClient(app_config["config"]["mongodb_uri"])
    await mongo_client.ping()
    app_config["config"]["mongo_client"] = mongo_client
    # create postgres connection
    max_workers = get_from_dict(app_config, "config.temporal.max_workers", 50)

    pg_pool = await asyncpg.create_pool(
        dsn=get_from_dict(app_config, "config.postgres_uri"),
        min_size=max_workers,
        max_size=max_workers,
    )

    async with pg_pool.acquire() as conn:
        await conn.execute("SELECT 1")

    app_config["postgres"] = pg_pool

    # do whatever else
    # store those shared elements on app_config

    # read all taxonomies
    app_config["taxonomies"] = dict()
    tax_path = app_config["config"]["taxonomies_path"]
    for file in os.listdir(tax_path):
        if ".tax" == file[-4:]:
            taxonomy_graph = load_taxonomy(
                os.path.join(tax_path, file), return_all=False, verbose=True
            )
            if taxonomy_graph.name != file[:-4]:
                print(
                    f'WARNING : Taxonomy file name is different from taxonomy graph name ("{file[:-4]}" vs "{taxonomy_graph.name}"). Using GRAPH NAME as taxonomy name.'
                )
            app_config["taxonomies"][taxonomy_graph.name] = taxonomy_graph

    return app_config


def get_app_config() -> dict:
    """
    Returns the global app config
    :return:
    """
    return app_config


def get_app_logger(name: str):
    return get_logger(name, get_from_dict(app_config, "config.log_level"))
