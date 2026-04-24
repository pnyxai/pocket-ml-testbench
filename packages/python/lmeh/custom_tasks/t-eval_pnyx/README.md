# T-Eval Dataset Task --- PNYX

This is task based on the T-Eval dataset. 
The evaluation procedure differs from the original paper, here we use more strict matching for calls. The semantic matches for API calling was not correct for many examples (i.e. API names cannot be soft-matched in the real world). Also we removed threshold-based semantic matching for the `review` task, replacing it with [A-VERT](https://github.com/pnyxai/a-vert) which is more robust and threshold-free semantic matching.

Currently we support only a subset of tasks, focusing on `json` and `tool_call` based responses and non-semantic matching.

To execute this task with `lm-eval` it is currently needed to use the [PnyxAI branch of our lm-eval fork](https://github.com/pnyxai/lm-evaluation-harness/tree/PnyxAI). It includes (among other things) two important PRs merged, [#3684](https://github.com/EleutherAI/lm-evaluation-harness/pull/3684) for injecting tools into the tests for OpenAI API format, and [#3685](https://github.com/EleutherAI/lm-evaluation-harness/pull/3685) for surfacing model's tool calls for evaluation.

## Original Dataset Paper
T-Eval: Evaluating the Tool Utilization Capability of Large Language Models Step by Step
https://arxiv.org/abs/2312.14033

Large language models (LLM) have achieved remarkable performance on various NLP tasks and are augmented by tools for broader applications. Yet, how to evaluate and analyze the tool-utilization capability of LLMs is still under-explored. In contrast to previous works that evaluate models holistically, we comprehensively decompose the tool utilization into multiple sub-processes, including instruction following, planning, reasoning, retrieval, understanding, and review. Based on that, we further introduce T-Eval to evaluate the tool utilization capability step by step. T-Eval disentangles the tool utilization evaluation into several sub-domains along model capabilities, facilitating the inner understanding of both holistic and isolated competency of LLMs. We conduct extensive experiments on T-Eval and in-depth analysis of various LLMs. T-Eval not only exhibits consistency with the outcome-oriented evaluation but also provides a more fine-grained analysis of the capabilities of LLMs, providing a new perspective in LLM evaluation on tool-utilization ability. The benchmark will be available at [this https URL](https://github.com/open-compass/T-Eval). 


### Groups and Tasks

#### Groups

* Not part of a group yet.

#### Tasks

- `t-eval_pnyx_instruct-v2`: Strict call syntax matching using (deprecated) `functions` standards.
- `t-eval_pnyx_instruct_tool_native-v2`: Strict call syntax matching using OpenAI API's `tool_call` standards.
- `t-eval_pnyx_plan-json-v2`: Strict plan adherence based on Hungarian matching and Longest Increasing Subsequence matching using (deprecated) `functions` standards.
- `t-eval_pnyx_plan-tool_native-v2`:  Strict plan adherence based on Hungarian matching and Longest Increasing Subsequence matching using OpenAI API's `tool_call` standards.
- `t-eval_pnyx_plan-reason-retrieve-understand-json-v2`: Strict call syntax matching using (deprecated) `functions` standards.
- `t-eval_pnyx_plan-reason-retrieve-understand_tool_native-v2`: Strict call syntax matching using OpenAI API's `tool_call` standards.
- `t-eval_pnyx_review-str-v2`: A-VERT semantic matching (no actual tool-calling hapens in this task).


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
v1 13-MAR-2026: `t-eval_pnyx_instruct-v2`, `t-eval_pnyx_plan-json-v2`, `t-eval_pnyx_plan-reason-retrieve-understand-json-v2`, `t-eval_pnyx_review-str-v2`
v2 06-APR-2023: `t-eval_pnyx_instruct_tool_native-v2`, `t-eval_pnyx_plan-tool_native-v2`, `t-eval_pnyx_plan-reason-retrieve-understand_tool_native-v2`