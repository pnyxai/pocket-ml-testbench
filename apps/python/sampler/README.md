# Sampler App

The Sampler receives workflow requests from the Manager and creates the tasks that are consumed by the Requester. It is in charge of populating the `tasks`, `instances` and `prompts` collection with all the correct task data.

## [From HF OpenLLM Leadeabord - FAQ](https://huggingface.co/spaces/HuggingFaceH4/open_llm_leaderboard)

Selected Tasks were updated to follow the `lm-eval-harness` `v0.4.9` (commit `452749513f817315042df9286241a61051392470`).
Task that require `loglikelihood` were not included. In this sense, the following task were selected to follow as close as possible the `HF OpenLLM Leadeabord`.

* MMLU: mmlu_abstract_algebra,mmlu_anatomy,mmlu_astronomy,mmlu_business_ethics,mmlu_clinical_knowledge,mmlu_college_biology,mmlu_college_chemistry,mmlu_college_computer_science,mmlu_college_mathematics,mmlu_college_medicine,mmlu_college_physics,mmlu_computer_security,mmlu_conceptual_physics,mmlu_econometrics,mmlu_electrical_engineering,mmlu_elementary_mathematics,mmlu_formal_logic,mmlu_global_facts,mmlu_high_school_biology,mmlu_high_school_chemistry,mmlu_high_school_computer_science,mmlu_high_school_european_history,mmlu_high_school_geography,mmlu_high_school_government_and_politics,mmlu_high_school_macroeconomics,mmlu_high_school_mathematics,mmlu_high_school_microeconomics,mmlu_high_school_physics,mmlu_high_school_psychology,mmlu_high_school_statistics,mmlu_high_school_us_history,mmlu_high_school_world_history,mmlu_human_aging,mmlu_human_sexuality,mmlu_international_law,mmlu_jurisprudence,mmlu_logical_fallacies,mmlu_machine_learning,mmlu_management,mmlu_marketing,mmlu_medical_genetics,mmlu_miscellaneous,mmlu_moral_disputes,mmlu_moral_scenarios,mmlu_nutrition,mmlu_philosophy,mmlu_prehistory,mmlu_professional_accounting,mmlu_professional_law,mmlu_professional_medicine,mmlu_professional_psychology,mmlu_public_relations,mmlu_security_studies,mmlu_sociology,mmlu_us_foreign_policy,mmlu_virology,mmlu_world_religions

* MMLUP-PRO: mmlu_pro-category_other, mmlu_pro-category_physics, mmlu_pro-category_chemistry, mmlu_pro-category_biology, mmlu_pro-category_psychology, mmlu_pro-category_health, mmlu_pro-category_business, mmlu_pro-category_law, mmlu_pro-category_history, mmlu_pro-category_philosophy, mmlu_pro-category_economics, mmlu_pro-category_math, mmlu_pro-category_engineering, mmlu_pro-category_computer-science

* BBH (CoT w/fewshots):bbh_cot_fewshot_tracking_shuffled_objects_three_objects, bbh_cot_fewshot_tracking_shuffled_objects_five_objects, bbh_cot_fewshot_tracking_shuffled_objects_seven_objects, bbh_cot_fewshot_dyck_languages, bbh_cot_fewshot_word_sorting, bbh_cot_fewshot_object_counting, bbh_cot_fewshot_reasoning_about_colored_objects, bbh_cot_fewshot_multistep_arithmetic_two, bbh_cot_fewshot_penguins_in_a_table, bbh_cot_fewshot_movie_recommendation, bbh_cot_fewshot_navigate, bbh_cot_fewshot_logical_deduction_three_objects, bbh_cot_fewshot_logical_deduction_five_objects, bbh_cot_fewshot_logical_deduction_seven_objects, bbh_cot_fewshot_causal_judgement, bbh_cot_fewshot_date_understanding, bbh_cot_fewshot_temporal_sequences, bbh_cot_fewshot_formal_fallacies, bbh_cot_fewshot_boolean_expressions, bbh_cot_fewshot_sports_understanding, bbh_cot_fewshot_disambiguation_qa, bbh_cot_fewshot_hyperbaton, bbh_cot_fewshot_salient_translation_error_detection, bbh_cot_fewshot_snarks, bbh_cot_fewshot_web_of_lies, bbh_cot_fewshot_ruin_names

* leaderboard_math: leaderboard_math_algebra_hard, leaderboard_math_counting_and_prob_hard, leaderboard_math_geometry_hard, leaderboard_math_intermediate_algebra_hard, leaderboard_math_num_theory_hard, leaderboard_math_prealgebra_hard, leaderboard_math_precalculus_hard

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
* `SamplerAPI` & `EvaluatorAPI`: Classe that mimic partially `TemplateAPI` from `lm-eval-harness`, depending the stage of the request (`sample` or `evaluate`) and the `path` of the request (`/v1/completions` or `/v1/chat/completions`).
    * `SamplerAPI` is a base class that is used to generate the `SamplerCompletionAPI` and `SamplerChatCompletionAPI`. Where each one generates the request in the format of `CompletionRequest` or `ChatCompletionRequest` when `_create_payload_custom` is called.
    * `EvaluatorAPI` similarly to `SamplerAPI` case that act as a base class for `EvaluatorCompletion` and `EvaluatorChatCompletion`. In `model_call` it convert the response in the format of `CompletionResponse` or `ChatCompletionResponse` into a dict. Then, responses are then parsed via `parse_generations`, a method inherited from `LocalCompletionsAPI` or `LocalChatCompletion`, depending if the request if the class is `EvaluatorCompletion` or `EvaluatorChatCompletion`.


Instead on generate request and take responses, both `loglikelihood` (current not implemented) and `generate_until` methods instantiate `CompletionRequest` or `ChatCompletionRequest`. The lasts are classes used as a proxy to generate the `data` field of a RPC request that is saved in Mongo.

**generator.py**
* `get_configurable_task`: A function to return only `ConfigurableTask` based on the `task_manager`. 
    * If `task_manager` is `TaskManager`, then all samples from all splits are part of the dataset. 
    * If `task_manager` is `PocketNetworkTaskManager`, random samples are generated based on the configuration split and the blacklist provided in `pocket_args`.
* `generate_requests`: A functions that hierarchically save in MongoDB the structure of `Task`->`Instances`->`Prompts`.


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