# Notes:

## [From HF OpenLLM Leadeabord - FAQ](https://huggingface.co/spaces/HuggingFaceH4/open_llm_leaderboard)

Selected Tasks were updated to follow the `lm-eval-harness` commit `7d9922c80114218eaf43975b7655bb48cda84f50`. In this sense, a closer version of `OpenLLM Leadeabord` would be as follow:

* ARC: arc_challenge

* HellaSwag: hellaswag

* TruthfulQA:truthfulqa_mc2

* MMLU: mmlu_abstract_algebra,mmlu_anatomy,mmlu_astronomy,mmlu_business_ethics,mmlu_clinical_knowledge,mmlu_college_biology,mmlu_college_chemistry,mmlu_college_computer_science,mmlu_college_mathematics,mmlu_college_medicine,mmlu_college_physics,mmlu_computer_security,mmlu_conceptual_physics,mmlu_econometrics,mmlu_electrical_engineering,mmlu_elementary_mathematics,mmlu_formal_logic,mmlu_global_facts,mmlu_high_school_biology,mmlu_high_school_chemistry,mmlu_high_school_computer_science,mmlu_high_school_european_history,mmlu_high_school_geography,mmlu_high_school_government_and_politics,mmlu_high_school_macroeconomics,mmlu_high_school_mathematics,mmlu_high_school_microeconomics,mmlu_high_school_physics,mmlu_high_school_psychology,mmlu_high_school_statistics,mmlu_high_school_us_history,mmlu_high_school_world_history,mmlu_human_aging,mmlu_human_sexuality,mmlu_international_law,mmlu_jurisprudence,mmlu_logical_fallacies,mmlu_machine_learning,mmlu_management,mmlu_marketing,mmlu_medical_genetics,mmlu_miscellaneous,mmlu_moral_disputes,mmlu_moral_scenarios,mmlu_nutrition,mmlu_philosophy,mmlu_prehistory,mmlu_professional_accounting,mmlu_professional_law,mmlu_professional_medicine,mmlu_professional_psychology,mmlu_public_relations,mmlu_security_studies,mmlu_sociology,mmlu_us_foreign_policy,mmlu_virology,mmlu_world_religions

* Winogrande: winogrande

* GSM8k: gsm8k

## Regist lm-eval-harness Task & HF datasets

The present script is based in part of `simple_evaluate` from `lm-eval-harness`. In particular, it's needded to catch `task_disk`, and then insert `task_disk.dataset` in postgreSQL.

```bash
python3 apps/python/register/register.py \
--tasks <task_names> \
--dbname <dbname> \
--user <user> \
--password <password> \
--host <host> \
--port <port> \
--include_path <include_path> \
--verbosity <verbosity> \
```

# Install:

## Local

1. Install dependencies:
`sudo apt-get install libpq-dev gcc git` before install `requirements.txt`

2. `pip install -r requirements.txt`

## Docker

1. Build with `./build.sh`

# Run

# Local

You can regist above task in postgreSQL running:

```bash
python3 apps/python/register/register.py \
--tasks arc_challenge,hellaswag,truthfulqa_mc2,mmlu_abstract_algebra,mmlu_anatomy,mmlu_astronomy,mmlu_business_ethics,mmlu_clinical_knowledge,mmlu_college_biology,mmlu_college_chemistry,mmlu_college_computer_science,mmlu_college_mathematics,mmlu_college_medicine,mmlu_college_physics,mmlu_computer_security,mmlu_conceptual_physics,mmlu_econometrics,mmlu_electrical_engineering,mmlu_elementary_mathematics,mmlu_formal_logic,mmlu_global_facts,mmlu_high_school_biology,mmlu_high_school_chemistry,mmlu_high_school_computer_science,mmlu_high_school_european_history,mmlu_high_school_geography,mmlu_high_school_government_and_politics,mmlu_high_school_macroeconomics,mmlu_high_school_mathematics,mmlu_high_school_microeconomics,mmlu_high_school_physics,mmlu_high_school_psychology,mmlu_high_school_statistics,mmlu_high_school_us_history,mmlu_high_school_world_history,mmlu_human_aging,mmlu_human_sexuality,mmlu_international_law,mmlu_jurisprudence,mmlu_logical_fallacies,mmlu_machine_learning,mmlu_management,mmlu_marketing,mmlu_medical_genetics,mmlu_miscellaneous,mmlu_moral_disputes,mmlu_moral_scenarios,mmlu_nutrition,mmlu_philosophy,mmlu_prehistory,mmlu_professional_accounting,mmlu_professional_law,mmlu_professional_medicine,mmlu_professional_psychology,mmlu_public_relations,mmlu_security_studies,mmlu_sociology,mmlu_us_foreign_policy,mmlu_virology,mmlu_world_religions,winogrande,gsm8k \
--dbname <dbname> \
--user <user> \
--password <user>> \
--host <user>> \
--port <user>> \
--verbosity <user>>
```

# Dockers

* (Optional): Prepare a postgreSQL from `infrastructure/postgresql` running `docker compose up`

1. Build register
`./build.sh`

2. Run

```bash
docker run -it --network host pocket_dataset_register \
/code/register.py \
--tasks arc_challenge,hellaswag,truthfulqa_mc2,mmlu_abstract_algebra,mmlu_anatomy,mmlu_astronomy,mmlu_business_ethics,mmlu_clinical_knowledge,mmlu_college_biology,mmlu_college_chemistry,mmlu_college_computer_science,mmlu_college_mathematics,mmlu_college_medicine,mmlu_college_physics,mmlu_computer_security,mmlu_conceptual_physics,mmlu_econometrics,mmlu_electrical_engineering,mmlu_elementary_mathematics,mmlu_formal_logic,mmlu_global_facts,mmlu_high_school_biology,mmlu_high_school_chemistry,mmlu_high_school_computer_science,mmlu_high_school_european_history,mmlu_high_school_geography,mmlu_high_school_government_and_politics,mmlu_high_school_macroeconomics,mmlu_high_school_mathematics,mmlu_high_school_microeconomics,mmlu_high_school_physics,mmlu_high_school_psychology,mmlu_high_school_statistics,mmlu_high_school_us_history,mmlu_high_school_world_history,mmlu_human_aging,mmlu_human_sexuality,mmlu_international_law,mmlu_jurisprudence,mmlu_logical_fallacies,mmlu_machine_learning,mmlu_management,mmlu_marketing,mmlu_medical_genetics,mmlu_miscellaneous,mmlu_moral_disputes,mmlu_moral_scenarios,mmlu_nutrition,mmlu_philosophy,mmlu_prehistory,mmlu_professional_accounting,mmlu_professional_law,mmlu_professional_medicine,mmlu_professional_psychology,mmlu_public_relations,mmlu_security_studies,mmlu_sociology,mmlu_us_foreign_policy,mmlu_virology,mmlu_world_religions,winogrande,gsm8k \
--dbname lm-evaluation-harness \
--user root \
--password root \
--host localhost \
--port 5432 \
--verbosity DEBUG
```

**Note**:If you have already downloaded HF datasets, mount them adding `-v path/to/huggingface/directory:/root/.cache/huggingface/` to avoid re-download.


### Accessing the DB with PG Admin

To explore the generated database, the PG Admin is available in the docker compose (`infrastructure/postgresql/docker-compose.yaml`).
To access the service just go to `127.0.0.1:5050` and use the credentials `admin@admin.com:admin`. 
Then in the PG Admin page click on `Add New Server` and fill the data:
General tab:
- Name: `pokt-ml-datasets`
Connection tab:
- Host Name: `postgres_container`
- Port: `5432`
- Maintenance database: `lm-evaluation-harness`
- Username: `root`
- Password: `root`