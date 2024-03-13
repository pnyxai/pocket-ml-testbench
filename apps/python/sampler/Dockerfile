FROM python:3.11-slim

RUN mkdir -p /code/src
WORKDIR /code/src

RUN apt-get update && apt-get install -y --no-install-recommends --allow-unauthenticated  \
    nano curl \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

COPY ./requirements.txt /code/requirements.txt
RUN pip install --no-cache-dir --upgrade -r /code/requirements.txt
RUN mkdir -p /mnt/secrets

COPY ./src /code/src

RUN ["python3", "sampler.py"]