# BigBenchHard (BBH)

- BBH (3-shots, multichoice)


A suite of 23 challenging BIG-Bench tasks which we call BIG-Bench Hard (BBH).
These are the task for which prior language model evaluations did not
outperform the average human-rater.

### Paper

Title: Challenging BIG-Bench Tasks and Whether Chain-of-Thought Can Solve Them

BIG-Bench (Srivastava et al., 2022) is a diverse evaluation suite that focuses on tasks believed to be beyond the capabilities of current language models. Language models have already made good progress on this benchmark, with the best model in the BIG-Bench paper outperforming average reported human-rater results on 65% of the BIG-Bench tasks via few-shot prompting. But on what tasks do language models fall short of average human-rater performance, and are those tasks actually unsolvable by current language models?
In this work, we focus on a suite of 23 challenging BIG-Bench tasks which we call BIG-Bench Hard (BBH). These are the task for which prior language model evaluations did not outperform the average human-rater. We find that applying chain-of-thought (CoT) prompting to BBH tasks enables PaLM to surpass the average human-rater performance on 10 of the 23 tasks, and Codex (code-davinci-002) to surpass the average human-rater performance on 17 of the 23 tasks. Since many tasks in BBH require multi-step reasoning, few-shot prompting without CoT, as done in the BIG-Bench evaluations (Srivastava et al., 2022), substantially underestimates the best performance and capabilities of language models, which is better captured via CoT prompting. As further analysis, we explore the interaction between CoT and model scale on BBH, finding that CoT enables emergent task performance on several BBH tasks with otherwise flat scaling curves.


- paper: https://huggingface.co/papers/2210.09261
- Homepage: https://github.com/suzgunmirac/BIG-Bench-Hard

### Citation

```
@article{suzgun2022challenging,
  title={Challenging BIG-Bench Tasks and Whether Chain-of-Thought Can Solve Them},
  author={Suzgun, Mirac and Scales, Nathan and Sch{\"a}rli, Nathanael and Gehrmann, Sebastian and Tay, Yi and Chung, Hyung Won and Chowdhery, Aakanksha and Le, Quoc V and Chi, Ed H and Zhou, Denny and and Wei, Jason},
  journal={arXiv preprint arXiv:2210.09261},
  year={2022}
}
```

### Groups

- `leaderboard_bbh`

### Tasks

- `leaderboard_bbh_boolean_expressions`
- `leaderboard_bbh_causal_judgement`
- `leaderboard_bbh_date_understanding`
- `leaderboard_bbh_disambiguation_qa`
- `leaderboard_bbh_formal_fallacies`
- `leaderboard_bbh_geometric_shapes`
- `leaderboard_bbh_hyperbaton`
- `leaderboard_bbh_logical_deduction_five_objects`
- `leaderboard_bbh_logical_deduction_seven_objects`
- `leaderboard_bbh_logical_deduction_three_objects`
- `leaderboard_bbh_movie_recommendation`
- `leaderboard_bbh_navigate`
- `leaderboard_bbh_object_counting`
- `leaderboard_bbh_penguins_in_a_table`
- `leaderboard_bbh_reasoning_about_colored_objects`
- `leaderboard_bbh_ruin_names`
- `leaderboard_bbh_salient_translation_error_detection`
- `leaderboard_bbh_snarks`
- `leaderboard_bbh_sports_understanding`
- `leaderboard_bbh_temporal_sequences`
- `leaderboard_bbh_tracking_shuffled_objects_five_objects`
- `leaderboard_bbh_tracking_shuffled_objects_seven_objects`
- `leaderboard_bbh_tracking_shuffled_objects_three_objects`
- `leaderboard_bbh_web_of_lies`
