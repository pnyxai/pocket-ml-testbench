FROM python:3.11.8-slim-bookworm

RUN apt-get update -y && \ 
    apt-get install -y libpq-dev gcc git

RUN mkdir -p /app
RUN mkdir -p /configs

COPY apps/python/sampler/pyproject.toml /app/pyproject.toml

WORKDIR /app

RUN pip3 install poetry

RUN poetry config virtualenvs.create false

RUN poetry install

COPY apps/python/sampler /app
COPY packages /app/packages

ENTRYPOINT ["poetry", "run", "python3", "worker/main.py"]