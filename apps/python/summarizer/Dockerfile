FROM pocket_ml_testbench_base_python_image:latest

# copy source code
COPY apps/python/summarizer /home/app/code
COPY packages /home/app/code/packages

# Copy taxonomies
COPY taxonomies /home/app/code/taxonomies


# install local packages (todo: implement our local packages to use this)
RUN poetry install --only main

# default entry point
ENTRYPOINT ["poetry", "run", "python3", "worker/main.py"]