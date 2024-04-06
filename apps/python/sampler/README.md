# pocket_lm_eval
Files that follow the structure of `lm-eval-harness`. The intention, for instance, is to avoid a fork.

## pocket_lm_eval - task
* **[New]** `PocketNetworkTaskManager`: A class based on `TaskManager`,  that is used to inject `pocket_args` into the `task.config.metadata`. 

## pocket_lm_eval - api
* **[New]** `PocketNetworkConfigurableTask`: A class based on `ConfigurableTask`, that retrieve samples from the sql database, based on `blacklist` id's & `uri` previously defined in `pocket_args`. In `PocketNetworkConfigurableTask.download` validations reagrding `training_split`, `validation_split`, `test_split` and `fewshot_split` are followed as pointed in the `lm-eval-harness- documentation. 

# generator

* **[New]** A functions `get_ConfigurableTask` to return only the random samples based of the configiguration split and the blacklist.

# Docker Run

```bash
docker run -it --network host pocket_sampler \
/code/sampler.py \
--pocket_args '{"hellaswag": {"address": "random", "blacklist": [49908, 59949], "qty": 15}}' \
--dbname lm-evaluation-harness \
--user admin \
--password admin \
--host localhost \
--port 5432 \
--verbosity DEBUG
```


### Create a copy of config.json and set the proper values for your environment
### Run
CONFIG_PATH=/your/path/to/config.json python worker/main.py
