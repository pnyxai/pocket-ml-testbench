import logging
import asyncpg
from packages.python.common.mongodb import MongoClient
from packages.python.common.utils import get_from_dict
from packages.python.logger.logger import get_logger
from lm_taxonomies import utils as txm_utils
from pathlib import Path

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
    logger = get_app_logger("summarizer_setup")
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
    tax_use = app_config["config"].get("taxonomies_use", None)
    if tax_use is not None:
        tax_use = tax_use.split(",")
    for file in os.listdir(tax_path):
        file_path = Path(os.path.join(tax_path, file))
        taxonmy_file_name = file_path.stem
        file_ext = file_path.suffix
        logger.debug(f"Checking: {taxonmy_file_name}{file_ext}.")
        if ".tax" == file_ext:
            if tax_use is None or file in tax_use:
                taxonomy_graph = txm_utils.load_taxonomy(
                    os.path.join(tax_path, file), return_all=False, verbose=True
                )
                if taxonomy_graph.name != taxonmy_file_name:
                    logger.debug(
                        f'WARNING : Taxonomy file name is different from taxonomy graph name ("{taxonmy_file_name}" vs "{taxonomy_graph.name}"). Using GRAPH NAME as taxonomy name.'
                    )
                app_config["taxonomies"][taxonomy_graph.name] = taxonomy_graph
                logger.info(
                    f"Added taxonomy to track: {taxonmy_file_name} ({taxonomy_graph.name})"
                )

    if tax_use is not None and len(app_config["taxonomies"]) == 0:
        raise ValueError(
            f"No valid taxonomy found in the provided list: {tax_path} / [{tax_use}]"
        )

    return app_config


def get_app_config() -> dict:
    """
    Returns the global app config
    :return:
    """
    return app_config


def get_app_logger(name: str):
    return get_logger(name, get_from_dict(app_config, "config.log_level"))
