FROM python:3.11.8-slim-bookworm

RUN apt-get update -y && \ 
    apt-get install -y libpq-dev gcc git

RUN mkdir -p /code
COPY ./requirements.txt /code/requirements.txt
RUN pip install --no-cache-dir -r /code/requirements.txt



# Mirror folder without unnecesary files
COPY ./register.py /code/register.py
COPY ./utils /code/utils

ENTRYPOINT ["python3"]