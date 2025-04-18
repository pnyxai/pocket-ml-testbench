# Sampler App

The Sampler receives workflow requests from the Manager and creates the tasks that are consumed by the Requester. It is in charge of populating the `tasks`, `instances` and `prompts` collection with all the correct task data.

## [From HF OpenLLM Leadeabord - FAQ](https://huggingface.co/spaces/HuggingFaceH4/open_llm_leaderboard)

Selected Tasks were updated to follow the `lm-eval-harness` commit `7d9922c80114218eaf43975b7655bb48cda84f50`. In this sense, a closer version of `OpenLLM Leadeabord` would be as follow:

* ARC: arc_challenge

* HellaSwag: hellaswag

* TruthfulQA:truthfulqa_mc2

* MMLU: mmlu_abstract_algebra,mmlu_anatomy,mmlu_astronomy,mmlu_business_ethics,mmlu_clinical_knowledge,mmlu_college_biology,mmlu_college_chemistry,mmlu_college_computer_science,mmlu_college_mathematics,mmlu_college_medicine,mmlu_college_physics,mmlu_computer_security,mmlu_conceptual_physics,mmlu_econometrics,mmlu_electrical_engineering,mmlu_elementary_mathematics,mmlu_formal_logic,mmlu_global_facts,mmlu_high_school_biology,mmlu_high_school_chemistry,mmlu_high_school_computer_science,mmlu_high_school_european_history,mmlu_high_school_geography,mmlu_high_school_government_and_politics,mmlu_high_school_macroeconomics,mmlu_high_school_mathematics,mmlu_high_school_microeconomics,mmlu_high_school_physics,mmlu_high_school_psychology,mmlu_high_school_statistics,mmlu_high_school_us_history,mmlu_high_school_world_history,mmlu_human_aging,mmlu_human_sexuality,mmlu_international_law,mmlu_jurisprudence,mmlu_logical_fallacies,mmlu_machine_learning,mmlu_management,mmlu_marketing,mmlu_medical_genetics,mmlu_miscellaneous,mmlu_moral_disputes,mmlu_moral_scenarios,mmlu_nutrition,mmlu_philosophy,mmlu_prehistory,mmlu_professional_accounting,mmlu_professional_law,mmlu_professional_medicine,mmlu_professional_psychology,mmlu_public_relations,mmlu_security_studies,mmlu_sociology,mmlu_us_foreign_policy,mmlu_virology,mmlu_world_religions

* Winogrande: winogrande

* GSM8k: gsm8k

# Activities - lm-evaluation-harness (LMEH)

## Utils
### pocket_lm_eval
Files that follow the structure of `lm-eval-harness`. The intention, for instance, is to avoid a fork.

**task**
* `PocketNetworkTaskManager`: A class based on `TaskManager`,  that is used to inject `pocket_args` into the `task.config.metadata`. 

**api**
* `PocketNetworkConfigurableTask`: A class based on `ConfigurableTask`, that retrieve samples from the sql database, based on `blacklist` id's & `uri` previously defined in `pocket_args`. In `PocketNetworkConfigurableTask.download` validations regarding `training_split`, `validation_split`, `test_split` and `fewshot_split` are followed as pointed in the `lm-eval-harness- documentation. 

    * `def build_all_requests` was modified in order to inject the Postgres document id into the `Instance.doc_id`.

**models**
* `PocketNetworkLM`:  A class that mimic partially `OpenaiChatCompletionsLM` from `lm-eval-harness`. Instead on generate request and take respobnses, both `loglikelihood` and `generate_until` methods instantiate `CompletionRequest`. The last is a class used as a proxy to generate the `data` field of a RPC request that is saved in Mongo.

**generator.py**
* `get_configurable_task`: A function to return only `ConfigurableTask` based on the `task_manager`. 
    * If `task_manager` is `TaskManager`, then all samples from all splits are part of the dataset. 
    * If `task_manager` is `PocketNetworkTaskManager`, random samples are generated based on the configuration split and the blacklist provided in `pocket_args`.
* `genererate_requests`: A functions that hierarchically save in MongoDB the structure of `Task`->`Instances`->`Prompts`.


### Accessing the DB with PG Admin

To explore the generated database, the PG Admin is available in the docker compose (`docker-compose/dependencies/docker-compose.yaml`).
To access the service just go to `127.0.0.1:5050` and use the credentials `admin@local.dev:admin`. 
Then in the PG Admin page click on `Add New Server` and fill the data:
General tab:
- Name: `pokt-ml-datasets`
Connection tab:
- Host Name: `postgres_container`
- Port: `5432`
- Maintenance database: `postgres`
- Username: `admin`
- Password: `admin`

# Docker

1. Generate image
```bash
apps/python/sampler/build.sh
```

2. Generate infrastructure following instructions in the [docker-compose README](../../../docker-compose/README.md)

3. Both for Register and Sampler workers generate a unique `config.json` like:

* Register

```json
{
  "postgres_uri": "postgresql://admin:admin@postgresql:5432/pocket-ml-testbench",
  "mongodb_uri": "mongodb://mongodb:27017/pocket-ml-testbench",
  "log_level": "DEBUG",
  "temporal": {
    "host": "temporal",
    "port": 7233,
    "namespace": "pocket-ml-testbench",
    "task_queue": "sampler-local",
    "max_workers": 10
  }
}
```

4. Run worker with:

```bash
cd docker-compose/app
docker compose up
```

5. Trigger Temporal Workflows:

* Register

```bash
temporal_docker workflow start \
 --task-queue sampler-local \
 --type Register \
 --input '{"framework": "lmeh", "tasks": "arc_challenge,hellaswag,truthfulqa_mc2,mmlu_abstract_algebra,mmlu_anatomy,mmlu_astronomy,mmlu_business_ethics,mmlu_clinical_knowledge,mmlu_college_biology,mmlu_college_chemistry,mmlu_college_computer_science,mmlu_college_mathematics,mmlu_college_medicine,mmlu_college_physics,mmlu_computer_security,mmlu_conceptual_physics,mmlu_econometrics,mmlu_electrical_engineering,mmlu_elementary_mathematics,mmlu_formal_logic,mmlu_global_facts,mmlu_high_school_biology,mmlu_high_school_chemistry,mmlu_high_school_computer_science,mmlu_high_school_european_history,mmlu_high_school_geography,mmlu_high_school_government_and_politics,mmlu_high_school_macroeconomics,mmlu_high_school_mathematics,mmlu_high_school_microeconomics,mmlu_high_school_physics,mmlu_high_school_psychology,mmlu_high_school_statistics,mmlu_high_school_us_history,mmlu_high_school_world_history,mmlu_human_aging,mmlu_human_sexuality,mmlu_international_law,mmlu_jurisprudence,mmlu_logical_fallacies,mmlu_machine_learning,mmlu_management,mmlu_marketing,mmlu_medical_genetics,mmlu_miscellaneous,mmlu_moral_disputes,mmlu_moral_scenarios,mmlu_nutrition,mmlu_philosophy,mmlu_prehistory,mmlu_professional_accounting,mmlu_professional_law,mmlu_professional_medicine,mmlu_professional_psychology,mmlu_public_relations,mmlu_security_studies,mmlu_sociology,mmlu_us_foreign_policy,mmlu_virology,mmlu_world_religions,winogrande,gsm8k"}' \
 --namespace pocket-ml-testbench
```

* Sampler

```bash
temporal_docker workflow start \
 --task-queue sampler-local \
 --type Sampler \
 --input '{"framework": "lmeh","tasks": "mmlu_high_school_macroeconomics", "requester_args": {"address": "SUPPLIER_ADDRESS", "service": "SERVICE_CODE"}, "blacklist": [11, 12], "qty": 15}' \
 --namespace pocket-ml-testbench
```

# Dev

1. Setup virtual env:
```bash
poetry install
poetry shell
```

2. Run worker
```bash
export CONFIG_PATH=/your/path/to/config.json
python3 worker/main.py
```
3. Generate request the same way as explained above