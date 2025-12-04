FROM pocket_ml_testbench_base_python_image:latest

# copy source code
COPY apps/python/evaluator /home/app/code
COPY packages /home/app/code/packages

USER root
RUN chown -R app:app /home/app/code/
USER app

# default entry point
ENTRYPOINT ["poetry", "run", "python3", "worker/main.py"]
