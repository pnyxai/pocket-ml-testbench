# DebugBench - PNYX

This is task based on the DebugBench paper's dataset.
**The test process is custom, not following the paper**

The test process will preset the LM a multiturn task (need to be run with `--apply_chat_template`):
- Turn 1: 
  - User asks the model the `question`
  - LM responds with `buggy_code`
- Turn 2 (This is the actual task to the LM):
  - User asks the model to correct the code, passing the `stderr` or saying it fails at tests.

The proposed solution from the LM is tested using `hf_evaluate`, therefore this task will requiere to execute arbitrary code, please take the necesary precautions.

## Dataset Paper
DebugBench: Evaluating Debugging Capability of Large Language Models
https://arxiv.org/abs/2401.04621

Large Language Models (LLMs) have demonstrated exceptional coding capability. However, as another critical component of programming proficiency, the debugging capability of LLMs remains relatively unexplored. Previous evaluations of LLMs' debugging ability are significantly limited by the risk of data leakage, the scale of the dataset, and the variety of tested bugs. To overcome these deficiencies, we introduce `DebugBench', an LLM debugging benchmark consisting of 4,253 instances. It covers four major bug categories and 18 minor types in C++, Java, and Python. To construct DebugBench, we collect code snippets from the LeetCode community, implant bugs into source data with GPT-4, and assure rigorous quality checks. We evaluate two commercial and four open-source models in a zero-shot scenario. We find that (1) while closed-source models exhibit inferior debugging performance compared to humans, open-source models relatively lower pass rate scores; (2) the complexity of debugging notably fluctuates depending on the bug category; (3) incorporating runtime feedback has a clear impact on debugging performance which is not always helpful. As an extension, we also compare LLM debugging and code generation, revealing a strong correlation between them for closed-source models. These findings will benefit the development of LLMs in debugging. 

### Groups and Tasks

#### Groups

* Not part of a group yet.

#### Tasks

- `debugbench_python_easy_sandbox_chat` pass@1, chat-template, with chat multiturns, easy examples
- `debugbench_python_medium_sandbox_chat` pass@1, chat-template, with chat multiturns, medium examples
- `debugbench_python_hard_sandbox_chat` pass@1, chat-template, with chat multiturns, hard examples


### Checklist

For adding novel benchmarks/datasets to the library:
* [ ] Is the task an existing benchmark in the literature?
  * [ ] Have you referenced the original paper that introduced the task?
  * [ ] If yes, does the original paper provide a reference implementation? If so, have you checked against the reference implementation and documented how to run such a test?


If other tasks on this dataset are already supported:
* [ ] Is the "Main" variant of this task clearly denoted?
* [ ] Have you provided a short sentence in a README on what each new variant adds / evaluates?
* [ ] Have you noted which, if any, published evaluation setups are matched by this variant?

### Changelog
v1 26-FEB-2026: `debugbench_python_easy_sandbox_chat`, `debugbench_python_medium_sandbox_chat`, `debugbench_python_hard_sandbox_chat`