import json
import asyncpg
import datetime
import decimal
import datasets

from app.app import get_app_logger
from temporalio.exceptions import ApplicationError

# Define the columns for the table
_ID_NAME = "__id"
_SPLIT_NAME = "__split"

POCKET_COLUMNS = {_ID_NAME: "INTEGER", _SPLIT_NAME: "TEXT"}

PRIMARY_KEY_DEF = f"PRIMARY KEY ({_ID_NAME}, {_SPLIT_NAME})"


async def checked_task(task_name: str, connection: asyncpg.Connection):
    """
    Check if a task is already registered in the registry table.

    Args:
    - task_name: Name of the task to be checked.
    - connection: asyncpg connection object.

    Returns:
    - True if the task is already registered, False otherwise.
    """
    # noinspection SqlNoDataSourceInspection
    record = await connection.fetchrow(
        """
        SELECT COUNT(*) FROM task_registry WHERE task_name = $1;
        """,
        task_name,
    )

    return record["count"] > 0


async def register_task(
    task_name: str, dataset_table_name: str, connection: asyncpg.Connection
):
    """
    Register a task in the registry task.

    Args:
    - task_name: Name of the task to be registered.
    - connection: asyncpg connection object.

    Returns:
    - None
    """
    # noinspection SqlNoDataSourceInspection
    await connection.execute(
        """
        INSERT INTO task_registry (task_name, dataset_table_name) VALUES ($1, $2) ON CONFLICT DO NOTHING;
        """,
        task_name,
        dataset_table_name,
    )


async def create_dataset_table(
    table_name: str, data: datasets.DatasetDict, connection: asyncpg.Connection
):
    """
    Create a PostgreSQL table based on a list of Python dictionaries.

    Args:
    - table_name: Name of the table to be created.
    - data: List of Python dictionaries where each dictionary represents a row in the table.
    - sample: A sample dictionary that represents a row in the table. This is used to infer the data types of the columns.
    - connection: asyncpg connection object.

    Returns:
    - None
    """
    eval_logger = get_app_logger("SQL")

    splits = list(data.keys())

    # Assumption: all splits have the same columns
    sample = data[splits[0]][0]

    # Extract column names and data types from the dictionaries
    columns = {}

    # Add manually k,v pairs "pocket_ID":INT, and "SPLIT":TEXT
    columns.update(POCKET_COLUMNS)

    for key, value in sample.items():
        if key not in columns:
            # If the column doesn't exist yet, infer its data type from the value
            columns[key] = infer_data_type(value)

    # Generate column definitions
    column_definitions = [
        f'"{column_name}" {data_type}' for column_name, data_type in columns.items()
    ]

    # Generate primary key definition
    column_definitions.append(PRIMARY_KEY_DEF)

    # Create a table statement
    # noinspection SqlNoDataSourceInspection
    column_definitions_str = ", ".join(column_definitions)
    create_table = (
        f'CREATE TABLE IF NOT EXISTS "{table_name}" ({column_definitions_str})'
    )

    # Insert data into the table statement
    column_names = ", ".join(f'"{column_name}"' for column_name in columns.keys())
    placeholders = ", ".join(f"${i + 1}" for i in range(len(columns)))
    insert_query = (
        f'INSERT INTO "{table_name}" ({column_names}) VALUES ({placeholders});'
    )

    await connection.execute(create_table)

    pocket_id = 0
    try:
        # Each k,v -> split, dataset
        data_rows = []
        for split, dataset in data.items():
            # Each row in the dataset
            for row in dataset:
                current_row = row.copy()
                current_row[_ID_NAME] = pocket_id
                current_row[_SPLIT_NAME] = split

                row_to_insert = list()
                for key in columns.keys():
                    val = current_row.get(key)
                    if isinstance(val, dict):
                        val = json.dumps(val)
                    row_to_insert.append(val)

                data_rows.append(tuple(row_to_insert))
                pocket_id += 1

        await connection.executemany(insert_query, data_rows)
    except Exception as error:
        error_msg = "Failed to inserting rows"
        eval_logger.error(error_msg, error=error, query=insert_query)
        raise ApplicationError(error_msg, error, type="SQLError", non_retryable=True)


# Function to infer PostgreSQL data type from python data type
def infer_data_type(value):
    mapping = {
        int: "INTEGER",
        bool: "BOOLEAN",
        float: "REAL",
        str: "TEXT",
        datetime.datetime: "TIMESTAMP",
        datetime.date: "DATE",
        datetime.time: "TIME",
        decimal.Decimal: "DECIMAL",
        list: "[]",
        dict: "JSON",
        bytes: "BYTEA",
    }
    v_type = mapping.get(type(value), "TEXT")
    # Handle lists
    if v_type == "[]":
        subvalue_type = mapping.get(type(value[0]), "TEXT")
        v_type = subvalue_type + v_type
    return v_type
