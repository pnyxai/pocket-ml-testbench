import time
import urllib
from contextlib import asynccontextmanager

from app.logger import init_logger
from motor.motor_asyncio import AsyncIOMotorClient


agg_get_nodes_ids = [{"$project": {"_id": 1}}]

agg_data_node = [
    {"$match": {"_id": ""}},
    {
        "$project": {
            "_id": 0,
            "address": "$address",
            "service": "$service",
            "last_seen_height": "$last_seen_height",
            "last_seen_time": "$last_seen_time",
        }
    },
]

agg_data_scores = [
    {"$match": {"task_data.node_id": ""}},
    {
        "$project": {
            "_id": 0,
            "framework": "$task_data.framework",
            "task": "$task_data.task",
            "last_seen": "$task_data.last_seen",
            "last_height": "$task_data.last_height",
            "mean": "$mean_scores",
            "median": "$median_scores",
            "std": "$std_scores",
            "num": "$circ_buffer_control.num_samples",
            "mean_times": "$mean_times",
            "median_times": "$median_times",
            "std_times": "$std_times",
            "error_rate": "$error_rate",
        }
    },
]

logger = init_logger(__name__)


class PoktMongodb:
    def __init__(self, MONGO_URI, VERBOSE=True):
        # Connect to mongodb server
        self.MONGO_URI = MONGO_URI
        self._uri = self.MONGO_URI
        self._parsed_uri = urllib.parse.urlparse(self.MONGO_URI)
        self._client = AsyncIOMotorClient(self.MONGO_URI)
        self._uri_db_name = self._parsed_uri.path[1:]

        # set vars
        self.VERBOSE = VERBOSE

    @property
    def db(self):
        return self._client[self._uri_db_name]

    @property
    def client(self):
        return self._client

    @property
    def db_name(self):
        return self._uri_db_name

    async def ping(self):
        return self._client.admin.command("ping")

    @asynccontextmanager
    async def start_session(self, *args, **kwargs):
        session = await self._client.start_session(*args, **kwargs)
        try:
            yield session
        finally:
            await session.end_session()

    @asynccontextmanager
    async def start_transaction(self, *args, **kwargs):
        async with self.start_session() as session:
            session.start_transaction(*args, **kwargs)
            try:
                yield session
            except Exception:
                # In contrast, in pymongo, start_transaction() returns a context manager
                # which can be used inside a with statement.
                # In motor, transactions should be manually committed or aborted.
                await session.abort_transaction()
                # let's bubble this one
                raise
            else:
                await session.commit_transaction()

    async def query(self, collection, aggregate, retries=10, waitTime=5, db="pokt"):
        tries = 0
        while True:
            try:
                result = list()
                async with self.start_transaction() as session:
                    async for document in self.db[collection].aggregate(
                        aggregate, session=session
                    ):
                        # Process each document here
                        result.append(document)
            except Exception as e:
                print(str(e) + "\n", flush=True)
                tries += 1
                if retries == tries:
                    raise ValueError("Error while trying to query DB.")
                time.sleep(retries * waitTime)
                continue
            break
        return result
