FROM pocket_ml_testbench_base_python_image:latest

# copy source code
COPY apps/python/sampler /home/app/code
COPY packages /home/app/code/packages

# install local packages (todo: implement our local packages to use this)
RUN poetry install --only main

# TODO : Remove this when a-vert is published!
COPY tilt/untracked/a_vert-0.1.0-py3-none-any.whl .
USER root
RUN chown -R app:app /home/app/code/
USER app
RUN poetry add ./a_vert-0.1.0-py3-none-any.whl

# default entry point
ENTRYPOINT ["poetry", "run", "python3", "worker/main.py"]