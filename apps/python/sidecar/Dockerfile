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

COPY apps/python/sidecar/requirements.txt /home/app/code/requirements.txt
RUN pip install --no-cache-dir --upgrade -r /home/app/code/requirements.txt


COPY apps/python/sidecar/app /home/app/code/app
COPY apps/python/sidecar/main.py /home/app/code/main.py
COPY packages /home/app/code/packages


CMD ["uvicorn", "main:app", "--host", "0.0.0.0" , "--port", "8000"]
