import json
import psycopg2
from psycopg2 import sql
import datetime
import decimal
import datasets

from app.app import get_app_logger
from temporalio.exceptions import ApplicationError

eval_logger = get_app_logger("sample")

# Define the columns for the table
_ID_NAME = "__id"
_SPLIT_NAME = "__split"

POCKET_COLUMNS = {
    _ID_NAME: "INTEGER",
    _SPLIT_NAME: "TEXT"
}

PRIMARY_KEY_DEF = sql.SQL(f"PRIMARY KEY ({_ID_NAME}, {_SPLIT_NAME})")


def create_task_table(connection: psycopg2.extensions.connection):
    """
    Create a table appending task, dataset name pairs.
    """
    with connection.cursor() as cursor:
        cursor.execute(
            """
            CREATE TABLE IF NOT EXISTS task_registry (
                task_name TEXT PRIMARY KEY,
                dataset_table_name TEXT
            )
            """
        )

    # connection.commit()


def checked_task(task_name: str, connection: psycopg2.extensions.connection):
    """
    Check if a task is already registered in the registry table.

    Args:
    - task_name: Name of the task to be checked.
    - connection: psycopg2 connection object.

    Returns:
    - True if the task is already registered, False otherwise.
    """

    with connection.cursor() as cursor:
        cursor.execute(
            """
            SELECT COUNT(*) FROM task_registry WHERE task_name = %s;
            """,
            (task_name,)
        )
        count = cursor.fetchone()[0]

    return count > 0


def register_task(task_name: str, dataset_table_name: str, connection: psycopg2.extensions.connection):
    """
    Register a task in the registry task.

    Args:
    - task_name: Name of the task to be registered.
    - connection: psycopg2 connection object.

    Returns:
    - None
    """

    with connection.cursor() as cursor:
        cursor.execute(
            """
            INSERT INTO task_registry (task_name, dataset_table_name) VALUES (%s, %s) ON CONFLICT DO NOTHING;
            """,
            (task_name, dataset_table_name)
        )

    # connection.commit()


def create_dataset_table(table_name: str, data: datasets.DatasetDict, connection: psycopg2.extensions.connection):
    """
    Create a PostgreSQL table based on a list of Python dictionaries.

    Args:
    - table_name: Name of the table to be created.
    - data: List of Python dictionaries where each dictionary represents a row in the table.
    - sample: A sample dictionary that represents a row in the table. This is used to infer the data types of the columns.
    - connection: psycopg2 connection object.

    Returns:
    - None
    """
    splits = list(data.keys())
    # Asummption: all splits have the same columns
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
        sql.SQL("{} {}").format(
            sql.Identifier(column_name),
            sql.SQL(data_type)
        )
        for column_name, data_type in columns.items()
    ]
    ## Generate primary key definition
    column_definitions.append(PRIMARY_KEY_DEF)
    # Create the table
    with connection.cursor() as cursor:
        cursor.execute(sql.SQL("CREATE TABLE IF NOT EXISTS {} ({})").format(
            sql.Identifier(table_name),
            sql.SQL(', ').join(column_definitions)
        ))
    connection.commit()

    # Insert data into the table
    insert_query = sql.SQL("INSERT INTO {} ({}) VALUES ({});").format(
        sql.Identifier(table_name),
        sql.SQL(', ').join(map(sql.Identifier, columns.keys())),
        sql.SQL(', ').join([sql.Placeholder()] * len(columns))
    )

    with connection.cursor() as cursor:
        pocket_id = 0
        # Each k,v -> split, dataset 
        for split, dataset in data.items():
            # Each row in the dataset
            for row in dataset:
                current_row = row.copy()
                current_row[_ID_NAME] = pocket_id
                current_row[_SPLIT_NAME] = split
                try:
                    cursor.execute(insert_query, [
                        current_row.get(key) if not isinstance(current_row.get(key), dict) else json.dumps(
                            current_row.get(key)) for key in columns.keys()])
                except Exception as e:
                    eval_logger.error(f"Error inserting:", row=current_row)
                    raise ApplicationError(f"Error: {e}, \nrow: {current_row}")
                pocket_id += 1
    connection.commit()


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
        bytes: "BYTEA"
    }
    v_type = mapping.get(type(value), "TEXT")
    # Handle lists
    if v_type == "[]":
        subvalue_type = mapping.get(type(value[0]), "TEXT")
        v_type = subvalue_type + v_type
    return v_type
