# MATH-hard

- Math-lvl-5 (4-shots, generative, minerva version)

This is the 4 shots variant of minerva math but only keeping the level 5 questions.

### Paper

Title: Measuring Mathematical Problem Solving With the MATH Dataset

Many intellectual endeavors require mathematical problem solving, but this
skill remains beyond the capabilities of computers. To measure this ability in
machine learning models, we introduce MATH, a new dataset of 12,500 challenging
competition mathematics problems. Each problem in MATH has a full step-by-step
solution which can be used to teach models to generate answer derivations and
explanations.

NOTE: The few-shot and the generated answer extraction is based on the
[Minerva](https://arxiv.org/abs/2206.14858) and exact match equivalence is
calculated using the `sympy` library. This requires additional dependencies,
which can be installed via the `lm-eval[math]` extra.

- Paper: https://huggingface.co/papers/2103.03874
- Homepage: https://github.com/hendrycks/math


### Citation

```
@article{hendrycksmath2021,
  title={Measuring Mathematical Problem Solving With the MATH Dataset},
  author={Dan Hendrycks and Collin Burns and Saurav Kadavath and Akul Arora and Steven Basart and Eric Tang and Dawn Song and Jacob Steinhardt},
  journal={NeurIPS},
  year={2021}
}
@misc{2206.14858,
Author = {Aitor Lewkowycz and Anders Andreassen and David Dohan and Ethan Dye and Henryk Michalewski and Vinay Ramasesh and Ambrose Slone and Cem Anil and Imanol Schlag and Theo Gutman-Solo and Yuhuai Wu and Behnam Neyshabur and Guy Gur-Ari and Vedant Misra},
Title = {Solving Quantitative Reasoning Problems with Language Models},
Year = {2022},
Eprint = {arXiv:2206.14858},
}
```

### Groups

- `leaderboard_math_hard`

### Tasks

- `leaderboard_math_algebra_hard`
- `leaderboard_math_counting_and_prob_hard`
- `leaderboard_math_geometry_hard`
- `leaderboard_math_intermediate_algebra_hard`
- `leaderboard_math_num_theory_hard`
- `leaderboard_math_prealgebra_hard`
- `leaderboard_math_precalculus_hard`

