FROM python:3.11.8-slim-bookworm

RUN apt-get update -y && \
    apt-get install -y libpq-dev gcc git curl graphviz libgraphviz-dev pkg-config && \
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
# Install uv
ENV UV_INSTALL_DIR=/usr/local/bin/
RUN curl -LsSf https://astral.sh/uv/install.sh | sh && chown app:app /usr/local/bin/uv

# copy poetry dependencies files
COPY apps/python/base_image/pyproject.toml /home/app/code/pyproject.toml
RUN chown app:app /home/app/code/pyproject.toml

# set app user as the user that will runs the commands below
USER app

# Clone the PNYX fork of lm-eval repo to get the necesary PRs merged in
RUN echo "4"
RUN git clone https://github.com/pnyxai/lm-evaluation-harness.git && \
    cd lm-evaluation-harness && \
    git checkout PnyxAI
# TODO : Replace the checkout branch for a tag to avoid no re-downloads on updates
RUN cd lm-evaluation-harness && uv build

# Clone the taxonomy repository to get the taxonomy files
RUN git clone https://github.com/pnyxai/lm-taxonomies.git && \
    cd lm-taxonomies && \
    git checkout v0.1.1
RUN mv ./lm-taxonomies/taxonomies /home/app/code/taxonomies

# Build the package too (not yet in pip)
RUN cd lm-taxonomies && uv build

# install only external dependencies without source code
RUN poetry  install --only main --no-root --no-directory

# Clone the a-vert repot to get the tasks
RUN git clone https://github.com/pnyxai/a-vert.git && \
    AVERT_VER=$(poetry show a-vert  | grep ' version .*' | cut --delimiter=: --fields=2 | cut --delimiter=' ' --fields=2) && \
    cd a-vert && \
    git checkout v${AVERT_VER}
RUN THIS_PATH=$(poetry env info -p) && mv ./a-vert/lm-eval_tasks/* $THIS_PATH/lib/python3.11/site-packages/lm_eval/tasks/
