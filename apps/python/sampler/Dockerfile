FROM python:3.11.8-slim-bookworm

RUN apt-get update -y && \ 
    apt-get install -y libpq-dev gcc git

RUN mkdir -p /code
COPY ./requirements_dev.txt /code/requirements_dev.txt
RUN pip install --no-cache-dir -r /code/requirements_dev.txt



# Mirror folder without unnecesary files
COPY ./sampler.py /code/sampler.py
COPY ./utils /code/utils

ENTRYPOINT ["python3"]