# LM Evaluation Harness (LMEH) for Pocket Network

## Overview

This module (`packages/python/lmeh`) is a Pocket Network-specific adapter layer on top of the **forked** [EleutherAI/lm-evaluation-harness](https://github.com/EleutherAI/lm-evaluation-harness). It integrates LLM benchmark evaluation with a distributed compute network (Pocket Network) by adding persistent storage, asynchronous sampling, and distributed evaluation capabilities.

The module bridges standard LM evaluation tasks (from lm-eval) with Pocket Network's workflow orchestration, allowing tasks to be:
1. **Registered** (task definition → SQL database)
2. **Sampled** (dataset selection + prompt generation → MongoDB)
3. **Evaluated** (LM inference + metric computation → MongoDB results)

---

## Module Structure

```
packages/python/lmeh/
├── pocket_lm_eval/              # Core Pocket Network task abstractions
│   ├── tasks/__init__.py         # PocketNetworkTaskManager (task indexing & loading)
│   ├── api/task.py               # PocketNetworkConfigurableTask & variants (persistence)
│   └── models/pocket_network.py  # LM API wrappers (sampler & evaluator)
├── utils/
│   ├── common.py                 # Task manager instantiation helper
│   ├── sql.py                    # PostgreSQL task registry & dataset I/O
│   ├── mongodb.py                # MongoDB task/prompt/result persistence
│   ├── generator.py              # Request generation (sampling) & evaluation pipeline
│   ├── task_config.py            # Task configuration overrides
│   ├── mongo_aggrs.py            # MongoDB aggregation pipelines
│   ├── tokenizers.py             # Tokenizer caching & management
│   └── __init__.py
├── custom_tasks/                 # Custom benchmark tasks (YAML definitions)
└── ARCHITECTURE.md               # This file
```

---

## How It Works

### 1. Task Manager: `PocketNetworkTaskManager` (tasks/__init__.py)

**Purpose:** Load task definitions and instantiate Pocket Network–aware task objects.

**Key Class:** `PocketNetworkTaskManager(TaskManager)`

Extends the standard lm-eval `TaskManager` to:
- **Intercept task loading** via `_load_spec()` override
- **Inject Pocket metadata** (workflow args, doc IDs, etc.) into task configs
- **Select task class by stage**: 
  - **Register** / **Sample** stages → `PocketNetworkConfigurableTask` (read-only, SQL-backed)
  - **Evaluate** stage → `EvaluatePocketNetworkConfigurableTask` (computes metrics)

**Flow:**
```
TaskManager.load_task_or_group(["task_name"])
  → _load_spec("task_name")  [overridden by Pocket]
    → _build_pocket_task()
      → Resolve YAML config from index
      → Inject {pocket_args, metadata}
      → Instantiate PocketNetworkConfigurableTask
```

**Key exports:**
- `TASK_MANAGER_REGISTER_STAGE` / `TASK_MANAGER_SAMPLE_STAGE` / `TASK_MANAGER_EVALUATE_STAGE` — workflow stage constants
- `get_task_manager()` in `utils/common.py` — factory to instantiate with DB connections and logging

---

### 2. Task Persistence: `PocketNetworkConfigurableTask` (api/task.py)

**Purpose:** Extend lm-eval's `ConfigurableTask` with SQL and MongoDB persistence.

**Key Classes:**
- `PocketNetworkConfigurableTask` — Base task for register/sample workflows
- `EvaluatePocketNetworkConfigurableTask` — Extends base with metric computation
- `SqlDatasetSaver` — Async PostgreSQL table creation and dataset insertion
- `SqlDatasetLoader` — Async dataset fetch from PostgreSQL

**Core Methods:**

| Method | Stage | Purpose |
|--------|-------|---------|
| `save_to_sql()` | Register | Create table, insert dataset rows (with row IDs for sampling) |
| `load_from_sql()` | Sample / Evaluate | Fetch dataset from PostgreSQL, hydrate `self.dataset` |
| `post_download()` | Both | Initialize filters, sampler, prompt config (mirrors parent's `__init__` logic) |
| `fewshot_docs()`, `doc_to_text()`, etc. | Evaluate | Inherited from `ConfigurableTask`; used during metric computation |

**Key Attributes (set by `PocketNetworkTaskManager`):**
```python
self.config.metadata["pocket_args"]  # PocketNetworkTaskRequest with doc_ids, qty, etc.
self.fewshot_cfg                     # FewshotConfig dataclass (few-shot formatting)
self.postgres_conn                   # asyncpg Connection for SQL I/O
self.eval_logger                     # Logger instance
self.hf_token                        # HuggingFace token for dataset downloads
```

**Important Detail:** The parent `ConfigurableTask.__init__` is **not called** (no `super().__init__()`). Instead, the Pocket version manually replicates the parent's initialization logic. This is intentional — it allows deferring dataset download until `load_from_sql()` is called (asynchronously) rather than during `__init__`. The tradeoff is that **all parent init updates must be manually mirrored** (e.g., `self.fewshot_cfg`, metric setup, etc.).

---

### 3. Generation & Evaluation: `generator.py`

**Purpose:** Orchestrate prompt generation (sampling) and metric computation (evaluation).

**Key Functions:**

| Function | Stage | Input | Output |
|----------|-------|-------|--------|
| `get_configurable_task()` | Register / Sample / Evaluate | Task names + overrides | `{task_name: Task}` dict |
| `generate_requests()` | Sample | Task + LM + limits | Generates instances, stores prompts in MongoDB |
| `evaluate()` | Evaluate | Task + LM + results | Processes responses, computes metrics, stores results in MongoDB |

**Flow: `generate_requests()` (Sampling)**
```
1. Build all instances from dataset (with limits/filtering/few-shot)
2. For each instance:
   - Call LM API (via SamplerCompletionAPI / SamplerChatCompletionAPI)
   - Collect responses
3. Insert into MongoDB:
   - tasks: PocketNetworkMongoDBTask (task metadata)
   - instances: Instance objects (with doc_id, request types)
   - prompts: PocketNetworkMongoDBPrompt (prompt text, context length, timeout)
4. Return True
```

**Flow: `evaluate()` (Evaluation)**
```
1. Fetch instances & doc_ids from MongoDB (via get_task_data activity)
2. Load dataset from SQL (same task as sampler, but fresh)
3. For each instance:
   - Fetch stored LM responses from MongoDB
   - Call task.process_results(doc, responses)
   - Collect metrics (one per doc)
4. Aggregate metrics (per-task)
5. Insert into MongoDB:
   - results: PocketNetworkMongoDBResultNumerical (scores per doc)
6. Return True, "OK"
```

---

### 4. SQL Persistence: `sql.py`

**Purpose:** Registry and dataset persistence in PostgreSQL.

**Key Functions:**
- `checked_task(task_name, conn)` — Check if task already registered
- `register_task(task_name, table_name, conn)` — Insert into `task_registry`
- `get_task_with_dataset()` — Fetch registered task + table name

**Database Schema:**
```sql
task_registry(
  task_name: TEXT PRIMARY KEY,
  dataset_table_name: TEXT,
  registered_at: TIMESTAMP
)

[dynamic per-task tables]
{TABLE_NAME}(
  __id: INTEGER,
  __split: TEXT,
  [columns from dataset schema],
  PRIMARY KEY(__id, __split)
)
```

**Why PostgreSQL for datasets?**
- Persistent across workflow retries (avoid re-downloading from HF)
- Deterministic sampling by row ID (reproducibility)
- Supports partial sampling (quota-based, without re-downloading full dataset)

---

### 5. MongoDB Persistence: `mongodb.py`

**Purpose:** Workflow coordination, prompt/response storage, results aggregation.

**Key Collections:**
- `tasks` — Task metadata + instance counts
- `instances` — Per-doc evaluation metadata (filtering, sample IDs)
- `prompts` — Prompt text, context length, LM timeout per instance
- `responses` — LM responses (stored separately for large response handling)
- `results` — Final aggregated metrics per task (benchmark results)

**MongoOperator Class:**
- `get_doc_ids_by_task(task_id)` — Fetch sampled doc IDs for evaluation
- `get_task(task_id)` — Fetch task config/args from sampling phase
- `instance_to_dict(instance, task_id)` — Convert Instance to MongoDB doc
- `save_results()` — Insert result documents

**Why MongoDB for this?**
- Flexible schema (instances, responses can vary by task type)
- Atomic transactions (sampling → LM → evaluation pipeline)
- Aggregation pipelines for analysis (via `mongo_aggrs.py`)

---

## Workflow Integration

The three stages are orchestrated by Temporal workflows:

### Register Workflow (`apps/python/sampler/workflows/register.py`)
```
user request → register_task activity
                  ↓
               get_task_manager(stage=REGISTER)
                  ↓
               get_configurable_task()
                  ↓
               task.save_to_sql()  [creates table, inserts dataset]
                  ↓
               sql.register_task() [marks task as ready]
```

### Sample Workflow (`apps/python/sampler/workflows/sampler.py`)
```
user request → lmeh_sample activity
                  ↓
               get_task_manager(stage=SAMPLE, pocket_args=...)
                  ↓
               get_configurable_task()
                  ↓
               task.load_from_sql()  [fetch dataset from DB]
                  ↓
               generate_requests()   [LM API calls]
                  ↓
               MongoDB: tasks, instances, prompts
```

### Evaluate Workflow (`apps/python/evaluator/workflows/evaluator.py`)
```
user request → get_task_data activity [fetch task from sample]
                  ↓
               lmeh_evaluate activity
                  ↓
               get_task_manager(stage=EVALUATE)
                  ↓
               get_configurable_task()
                  ↓
               task.load_from_sql()
                  ↓
               evaluate()  [fetch responses, compute metrics]
                  ↓
               MongoDB: results
                  ↓
               manager workflow [process results]
```

---

## Key Interactions with Standard lm-eval

### Inheritance & Overrides

| Component | Parent | Override | Why |
|-----------|--------|----------|-----|
| `PocketNetworkTaskManager` | `TaskManager` | `_load_spec()` | Intercept task instantiation to inject Pocket metadata |
| `PocketNetworkConfigurableTask` | `ConfigurableTask` | Most methods | Add SQL I/O, defer dataset loading, inject metadata |
| Models (sampler/evaluator) | `APIModelAdapter` | Completion/Chat API | Add MongoDB result storage, handle Pocket-specific request format |

### What's NOT Overridden

- **Task interface** — All standard task methods work (`doc_to_text()`, `doc_to_target()`, `process_results()`, etc.)
- **Filtering & aggregation** — Standard lm-eval filters and aggregations apply
- **Few-shot logic** — Uses parent's `fewshot_docs()` and sampler selection
- **Metrics** — Standard lm-eval metric registry; no Pocket-specific metrics

**Result:** Pocket Network tasks are **100% compatible** with standard lm-eval tasks. You can swap any task definition (YAML or Python class) without modification.

---

## Configuration & Customization

### Task YAML (in `custom_tasks/`)
```yaml
task: example_task
dataset_path: wikitext
dataset_name: wikitext-2
num_fewshot: 0
output_type: generate_until
# Standard lm-eval fields...
```

No Pocket-specific YAML config needed — Pocket metadata is injected at runtime via `pocket_args`.

### Overrides (from workflow args)
```python
# In sample.py:
task_dict = get_configurable_task(
    tasks=["task_name"],
    num_fewshot=args.num_fewshot,           # Runtime override
    gen_kwargs=args.gen_kwargs,              # Runtime override
    task_manager=task_manager,               # Instance with DB conns
    # ...
)
```

### Async Operations
All SQL/MongoDB I/O is **async**:
- `await task.save_to_sql()`
- `await task.load_from_sql()`
- `await mongo_client.db["collection"].insert_many(...)`

This integrates with Temporal's async task execution model.

---

## Extending the Module

### Adding a New Task
1. Create YAML in `custom_tasks/your_task.yaml`
2. No Pocket changes needed — `PocketNetworkTaskManager` loads it automatically
3. Register via the Register workflow

### Adding a New LM API
1. Subclass `APIModelAdapter` in `pocket_lm_eval/models/pocket_network.py`
2. Override `generate_until()` / `loglikelihood()` as needed
3. Ensure responses are stored in MongoDB (see `MongoOperator`)

### Adding a New Persistence Backend
1. Create a new class in `utils/` (e.g., `redis.py`, `s3.py`)
2. Implement the same interface as `SqlDatasetLoader` / `SqlDatasetSaver`
3. Update `PocketNetworkConfigurableTask` to call your backend instead

---

## Testing & Debugging

### Local Testing (without Pocket Network)
```python
from packages.python.lmeh.utils.common import get_task_manager

task_manager = get_task_manager(
    tasks="your_task",
    include_path=None,
    verbosity="DEBUG",
    postgres_conn=None,  # Will fail on SQL calls, but okay for init testing
    stage="sample",
)
task_dict = task_manager.load_task_or_group(["your_task"])
print(task_dict["your_task"])  # Inspect the task object
```

### Inspecting MongoDB Results
```python
from packages.python.common.mongodb import MongoClient

client = MongoClient(...)
results = await client.db["results"].find({"task_id": ObjectId("...")}).to_list(None)
print(results[0])  # See the structure
```

### SQL Dataset Inspection
```sql
-- Connect to PostgreSQL
SELECT COUNT(*) FROM "{TABLE_NAME}";
SELECT * FROM "{TABLE_NAME}" LIMIT 5;
```

---

## Dependencies

**Key external packages:**
- `lm-eval` (forked, at `../lm-evaluation-harness`)
- `asyncpg` — PostgreSQL async driver
- `datasets` — HuggingFace datasets library
- `temporalio` — Temporal workflows

**Key internal packages:**
- `packages.python.protocol` — Protobuf definitions (task requests, MongoDB models)
- `packages.python.common.mongodb` — MongoDB client wrapper

---

## Summary

The LMEH module is a **stateful, distributed evaluation framework** that bridges lm-eval's task definitions with Pocket Network's workflow orchestration. It adds three key capabilities:

1. **Persistent task registry** (SQL) — No re-downloads, reproducible sampling
2. **Async generation & evaluation** — Integrates with Temporal workflows
3. **Distributed results** (MongoDB) — Aggregation and analysis across many evaluations

All while maintaining **100% compatibility** with standard lm-eval tasks and metrics.
