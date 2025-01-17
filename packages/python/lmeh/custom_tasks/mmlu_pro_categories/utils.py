from functools import partial


choices = [
    "A",
    "B",
    "C",
    "D",
    "E",
    "F",
    "G",
    "H",
    "I",
    "J",
    "K",
    "L",
    "M",
    "N",
    "O",
    "P"
]


# TODO: Implement CoT test
def format_cot_example(example, including_answer=True):
    prompt = "Question:\n"
    question = example["question"]
    options = example["options"]
    prompt += question + "\n"
    prompt += "Options:\n"
    for i, opt in enumerate(options):
        prompt += "{}. {}\n".format(choices[i], opt)
    if including_answer:
        cot_content = example["cot_content"].replace(
            "A: Let's think step by step.", "Answer: Let's think step by step."
        )
        prompt += cot_content + "\n\n"
    else:
        prompt += "Answer: Let's think step by step."
    return prompt

doc_to_text_CoT = partial(format_cot_example, including_answer=False)
fewshot_to_text_CoT = partial(format_cot_example, including_answer=True)

def format_example(example, including_answer=True):
    prompt = "Question:\n"
    question = example["question"]
    options = example["options"]
    prompt += question + "\n"
    prompt += "Options:\n"
    for i, opt in enumerate(options):
        prompt += "{}. {}\n".format(choices[i], opt)
    if including_answer:
        prompt += f"Answer: The answer is ({choices[example['answer_index']]})." + "\n\n"
    else:
        prompt += "Answer: "
    return prompt


doc_to_text = partial(format_example, including_answer=False)
fewshot_to_text = partial(format_example, including_answer=True)