# Musr

- Musr (0-shot, multichoice)


### Paper

Title: MuSR: Testing the Limits of Chain-of-thought with Multistep Soft
Reasoning  

While large language models (LLMs) equipped with techniques like
chain-of-thought prompting have demonstrated impressive capabilities, they
still fall short in their ability to reason robustly in complex settings.
However, evaluating LLM reasoning is challenging because system capabilities
continue to grow while benchmark datasets for tasks like logical deduction have
remained static. We introduce MuSR, a dataset for evaluating language models on
multistep soft reasoning tasks specified in a natural language narrative. This
dataset has two crucial features. First, it is created through a novel
neurosymbolic synthetic-to-natural generation algorithm, enabling the
construction of complex reasoning instances that challenge GPT-4 (e.g., murder
mysteries roughly 1000 words in length) and which can be scaled further as more
capable LLMs are released. Second, our dataset instances are free text
narratives corresponding to real-world domains of reasoning; this makes it
simultaneously much more challenging than other synthetically-crafted
benchmarks while remaining realistic and tractable for human annotators to
solve with high accuracy. We evaluate a range of LLMs and prompting techniques
on this dataset and characterize the gaps that remain for techniques like
chain-of-thought to perform robust reasoning.

- Paper: https://huggingface.co/papers/2310.16049
- Homepage: https://zayne-sprague.github.io/MuSR/

### Citation

```
@misc{sprague2024musrtestinglimitschainofthought,
      title={MuSR: Testing the Limits of Chain-of-thought with Multistep Soft
      Reasoning},
      author={Zayne Sprague and Xi Ye and Kaj Bostrom and Swarat Chaudhuri and Greg Durrett},
      year={2024},
      eprint={2310.16049},
      archivePrefix={arXiv},
      primaryClass={cs.CL},
      url={https://arxiv.org/abs/2310.16049},
}
```

### Groups

- `leaderboard_musr`

### Tasks

- `leaderboard_musr_murder_mysteries`
- `leaderboard_musr_object_placements`
- `leaderboard_musr_team_allocation`
