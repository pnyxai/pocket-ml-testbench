from functools import partial
from lm_eval.filters.extraction import Filter
from lm_eval.api.registry import register_filter
import re


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
    "P",
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
        prompt += (
            f"Answer: The answer is ({choices[example['answer_index']]})." + "\n\n"
        )
    else:
        prompt += "Answer: "
    return prompt


doc_to_text = partial(format_example, including_answer=False)
fewshot_to_text = partial(format_example, including_answer=True)


@register_filter("GetResponse")
class GetResponse(Filter):
    """ """

    def apply(self, resps, docs):
        filtered_resps = []

        for r, doc in zip(resps, docs):
            filtered = []
            for resp in r:
                if "</think>" in resp:
                    # Remove CoT content
                    resp = resp.split("</think>")[-1]
                else:
                    # Remove everything after double line jump
                    resp = resp.split("\n\n")[0]
                # Remove leading white spaces
                resp = resp.lstrip()
                # function to ignore right white spaces or line breaks
                resp = re.sub(r"\s+$", "", resp)
                # If there are things  between brackets, match those
                search = re.search("\(([^)]+)\)", resp)
                if search is not None:
                    resp = search.groups(0)[0]

                filtered.append(resp)
            filtered_resps.append(filtered)

        return filtered_resps
