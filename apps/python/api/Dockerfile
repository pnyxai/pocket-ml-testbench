FROM python:3.11.8-slim-bookworm

RUN apt-get update -y && \
    apt-get install -y libpq-dev gcc git curl && \
    groupadd -r app && \
    useradd -r -g app -d /home/app -s /sbin/nologin -c "Docker image user" app && \
    mkdir /home/app && \
    mkdir -p /home/app/code && \
    mkdir -p /home/app/configs && \
    chown -R app:app /home/app

# create default working directory
WORKDIR /home/app/code

COPY apps/python/api/requirements.txt /home/app/code/requirements.txt
RUN pip install --no-cache-dir --upgrade -r /home/app/code/requirements.txt


COPY apps/python/api/app /home/app/code/app
COPY apps/python/api/main.py /home/app/code/main.py


CMD ["uvicorn", "main:app", "--host", "0.0.0.0" , "--port", "3001"]
