import re
import json
from datasets import Dataset

# Select code executor
import evaluate as hf_evaluate
compute_ = dict()
compute_["python3"] = hf_evaluate.load("code_eval")


try:
    test_cases = ["assert add(2, 3)==5"]
    candidates = [["def add(a,b): return a*b"]]
    results = compute_['python3'].compute(references=test_cases, predictions=candidates, k=[1])
except Exception as e:
    raise e


def process_example(example, language):
    # Parse your fewshot_context/history to list of dicts
    # Assume "fewshot_context" is JSON string like '[{"role":"user","content":"..."},{"role":"assistant","content":"..."}]'
    # history = json.loads(example.get("fewshot_context", "[]"))

    history = list()
    # Add initial user question
    history.append(
        {
            "role": "user",
            "content": f"I need to solve the following using {language} code.\n{example['question']}",
        }
    )
    # Add wrong answer from assistant
    history.append(
        {
            "role": "assistant",
            "content": f"Okay, this code should solve that!\n\n```\n{example['buggy_code']}\n```",
        }
    )
    # Append final user prompt as last turn
    if example["stderr"] != "":
        # Add error correction request
        history.append(
            {
                "role": "user",
                "content": f"The code you provided is not working, it results in this error:\n{example['stderr']}\nPlease provide a corrected code block.",
            }
        )
    else:
        # Ask for correction because tests are failing
        history.append(
            {
                "role": "user",
                "content": f"The code you provided is not working, it is failing some tests I have.\nPlease review it and provide a corrected code block.",
            }
        )

    example["multi_turn"] = json.dumps(history)  # List of dicts for chat template
    return example


def process_docs_python(dataset: Dataset) -> Dataset:
    _process = lambda x: process_example(x, "Python3")
    return dataset.map(_process)

def pass_at_k_process(doc, results):
    global compute_
    
    result = {"pass@1": 0}

    # We support a single prediction currently
    prediction = results[0][0]
    if prediction.strip() == "" or prediction is None:
        return result
    
    assert doc["test_code"] is not None

    # Get sample language
    language = doc["language"]

    # Compute
    res = compute_[language].compute(
        references=[doc["test_code"]],
        predictions=[[prediction]],
        k=[1],
    )
    result["pass@1"] = res[0]['pass@1']

    return result


def build_predictions_chat_debugbench(
    resps: list[list[str]], docs: list[dict]
) -> list[list[str]]:
    out = list()
    for resp, doc in zip(resps, docs):
        initialization_code = doc["initialization_code"]
        out.append(list())
        if len(resp) == 0:
            out[-1].append(initialization_code + "\n")
        else:
            for r in resp:
                filtered = extract_longest_code_block(r)
                if filtered is not None:
                    out[-1].append(initialization_code + "\n" + filtered["content"])
                else:
                    out[-1].append(initialization_code + "\n" + r)
    return out


def extract_longest_code_block(text: str) -> dict | None:
    """
    Extracts the longest code block from text.

    A code block starts with ```language (e.g., ```python) and ends with ```.
    Returns dict with 'language', 'content', and 'length', or None if none found.
    """
    # Matches ```language followed by content until ```
    pattern = r"```([a-zA-Z0-9]+)\s*(.*?)\s*```"
    matches = re.findall(pattern, text, re.DOTALL)

    if not matches:
        return None

    # Select longest by content length
    longest = max(matches, key=lambda x: len(x[1].strip()))
    language, content = longest

    return {"language": language, "content": content, "length": len(content)}
