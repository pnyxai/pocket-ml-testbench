FROM python:3.11.8-slim-bookworm

RUN apt-get update -y && \
    apt-get install -y libpq-dev gcc git && \
    groupadd -r app && \
    useradd -r -g app -d /home/app -s /sbin/nologin -c "Docker image user" app && \
    mkdir /home/app && \
    mkdir -p /home/app/code && \
    mkdir -p /home/app/configs && \
    chown -R app:app /home/app

# create default working directory
WORKDIR /home/app/code

# install poetry as root and give app user access to it
RUN pip3 install poetry && chown app:app /usr/local/bin/poetry

# copy poetry dependencies files
COPY apps/python/sampler/pyproject.toml /home/app/code/pyproject.toml
COPY apps/python/sampler/poetry.lock /home/app/code/poetry.lock

# set app user as the user that will runs the commands below
USER app

# install only external dependencies without source code
RUN poetry  install --only main --no-root --no-directory

# copy source code
COPY apps/python/sampler /home/app/code
COPY packages /home/app/code/packages

# install local packages (todo: implement our local packages to use this)
RUN poetry install --only main

# default entry point
ENTRYPOINT ["poetry", "run", "python3", "worker/main.py"]