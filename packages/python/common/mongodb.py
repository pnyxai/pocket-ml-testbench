import urllib.parse
from contextlib import asynccontextmanager
from motor.motor_asyncio import AsyncIOMotorClient


class MongoClient:
    """
    MongoClient

    Class representing a MongoDB client.

    Attributes:
        _uri (str): The connection URI for the MongoDB client.
        _parsed_uri (urllib.ParseResult): The parsed connection URI.
        _client (AsyncIOMotorClient): The async MongoDB client.
        _uri_db_name (str): The name of the MongoDB database.

    Properties:
        db: Property representing the MongoDB database.
        client: Property representing the MongoDB client.
        db_name: Property representing the name of the MongoDB database.

    Methods:
        __init__(self, uri): Constructor method for MongoClient.
        ping(self): Asynchronously pings the MongoDB server.
        start_session(self, *args, **kwargs): Asynchronously starts a session with the MongoDB client.
        start_transaction(self, *args, **kwargs): Asynchronously starts a transaction with the MongoDB client.

    """
    def __init__(self, uri):
        self._uri = uri
        self._parsed_uri = urllib.parse.urlparse(uri)
        self._client = AsyncIOMotorClient(uri)
        self._uri_db_name = self._parsed_uri.path[1:]

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
        return self._client.admin.command('ping')

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
