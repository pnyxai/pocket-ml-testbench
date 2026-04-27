"""
Utility module for T-Eval task evaluation and response processing.

This module provides functions for:
1. Evaluating model responses across multiple evaluation tasks (instruct, plan, review, rru)
2. Building and processing predictions from language model outputs
3. Scoring and matching predictions against ground truth
4. Parsing responses in different formats (JSON, string templates)
5. Semantic and structural matching of function calls and plans

The module integrates with the A-VERT semantic evaluation system for enhanced matching
capabilities on subjective arguments like thoughts and queries.
"""

import re
import json
import ast
import numpy as np
import networkx as nx
from copy import deepcopy
import numbers

try:
    import a_vert

    AVERT_OK = True
except Exception as e:
    print(
        f"WARNING: `a_vert` library not foud. Task `t-eval_pnyx_review-str-v2` will be unavailable: {e}"
    )
    AVERT_OK = False

from string import Formatter

if AVERT_OK:
    # Setup A-VERT configuration from environment variables
    AVERT_CONFIG = a_vert.setup()
    # For backward compatibility, extract individual values
    ENHANCE = AVERT_CONFIG.enhance
    # Default instruction map
    default_instruction = {
        "default": "Find the document that better represents the meaning in the query. Check for any doubts about the question or options. Focus on exact numbers, dates, or symbols.",
    }
    if not AVERT_CONFIG.instruction_map:
        AVERT_CONFIG.instruction_map = default_instruction


def t_eval_instruct_match(doc, results):
    """
    Evaluate instruction-following task by matching parsed response against ground truth.

    This function scores how well a model followed instructions by checking if the
    parsed response (thought, action, args) matches the expected ground truth structure
    and values.

    Args:
        doc (dict): Document containing ground truth with keys:
            - 'ground_truth': JSON string of expected response structure
        results (list): List of predictions where results[0][0] is the first prediction dict with:
            - 'call_dict': Mapping of response fields to check
            - 'test_keys_dict': Template for expected fields
            - 'response': Parsed response (dict/object)

    Returns:
        dict: Evaluation result with key:
            - 'call_match': Binary match score (0 or 1)
    """
    score = 0
    # We support a single prediction currently
    prediction = results[0][0]
    if "failed_parse" not in prediction.keys():
        if len(prediction["call_dict"]) > 0:
            score = instruct_scorer(
                prediction["test_keys_dict"],
                prediction["call_dict"],
                json.loads(doc["ground_truth"]),
                prediction["response"],
            )

    return {"call_match": score}


def t_eval_plan_match(doc, results):
    """
    Evaluate planning task by matching predicted plan against ground truth plan.

    Calculates precision, recall, and F1 score between predicted and ground truth
    action sequences. Uses Hungarian matching and Longest Increasing Subsequence (LIS)
    for plan alignment. Does not enforce strict numerical matching since the prompt
    allows invented ID numbers.

    Args:
        doc (dict): Document containing ground truth with keys:
            - 'ground_truth': JSON string with 'answer' containing list of action dicts
        results (list): List of predictions where results[0][0] is the first prediction
            (list of action dicts with 'name', 'args', 'id' fields)

    Returns:
        dict: Evaluation metrics with keys:
            - 'plan_precision': Ratio of correctly matched predicted actions
            - 'plan_recall': Ratio of ground truth actions matched
            - 'plan_f1_score': Harmonic mean of precision and recall
    """
    result = {"plan_precision": 0, "plan_recall": 0, "plan_f1_score": 0}
    # We support a single prediction currently
    prediction = results[0][0]
    if not isinstance(prediction, list) and isinstance(prediction, dict):
        # Maybe this is a single-action plan
        prediction = [prediction]

    if "failed_parse" not in prediction[0].keys():
        # No strict numerical match since the prompt asks for these to be invented
        result_here = planing_scorer(
            prediction, json.loads(doc["ground_truth"]), strict_numerical_match=False
        )

        result["plan_precision"] = result_here["precision"]
        result["plan_recall"] = result_here["recall"]
        result["plan_f1_score"] = result_here["f1_score"]

    return result


def t_eval_review_match(doc, results):
    """
    Evaluate review/ranking task using A-VERT semantic similarity scoring.

    Compares a model's response (which should identify the correct option) against
    constructed A-VERT candidate groups using semantic embeddings. Calculates separate
    scores for correct and wrong option groups.

    Args:
        doc (dict): Document containing ground truth with keys:
            - 'ground_truth': JSON string with 'answer' containing correct option (A-H)
        results (list): List containing model response(s) as results[0] (str or list of responses)

    Returns:
        dict: A-VERT evaluation metrics with keys:
            - 'a-vert_correct_score': Semantic similarity score for correct option
            - 'a-vert_wrong_score': Semantic similarity score for wrong options
            - 'a-vert_match': Boolean indicating if correct score > wrong score
    """
    if not AVERT_OK:
        raise ValueError(
            "A-VERT not available, cannot run task: `t-eval_pnyx_review-str-v2`"
        )
    # Get a-vert response groups
    avert_groups = construct_review_response_groups(
        json.loads(doc["ground_truth"])["answer"]
    )
    # We support a single prediction currently
    prediction = results[0]

    # Process all candidate groups
    response_group_distribution, _ = (
        a_vert.processing.get_candidate_groups_embedings_ranking(
            prediction,
            avert_groups,
            AVERT_CONFIG,
            task="default",
        )
    )
    # Check if this is a match
    a_vert_match = True
    if response_group_distribution["correct"] < response_group_distribution["wrong"]:
        a_vert_match = False

    result = dict()
    result["a-vert_correct_score"] = response_group_distribution["correct"]
    result["a-vert_wrong_score"] = response_group_distribution["wrong"]
    result["a-vert_match"] = a_vert_match

    return result


def t_eval_rru_match(doc, results):
    """
    Evaluate reasoning with retrieval and use (RRU) task by matching function calls.

    Checks if the predicted function call matches the ground truth. Uses strict
    numerical matching only if all numerical arguments appear in the original prompt.

    Args:
        doc (dict): Document containing:
            - 'ground_truth': JSON string with expected call (name, args)
            - 'origin_prompt': JSON string with conversation history to check for numbers
        results (list): List of predictions where results[0][0] is the first prediction
            (dict with 'name' and 'args' fields)

    Returns:
        dict: Evaluation result with key:
            - 'call_match': Binary match score (0 or 1) based on function name and arguments
    """

    def get_rru_score(this_prediction):
        # Parse ground truth
        gt_parsed = json.loads(doc["ground_truth"])
        # Check if strict match is possible
        strict_num_match = check_strict_numerical_match(
            json.loads(doc["origin_prompt"]), gt_parsed["args"]
        )
        # Get score
        return call_matching(
            this_prediction, gt_parsed, strict_numerical_match=strict_num_match
        )

    # Default score
    result = {
        "call_match": 0,
    }
    # We support a single prediction currently
    prediction = results[0][0]

    # Check valid generation
    if isinstance(prediction, dict):
        if "failed_parse" in prediction.keys():
            return result
    elif isinstance(prediction, list):
        pass
    else:
        # Not a JSON, cannot evaluate
        return result

    if isinstance(prediction, list) and doc["response_format"] == "tool_calls":
        # When prompted with native tool calls, the model can propose multiple
        # tool calls, we will evaluate if any of those is correct since the
        # dataset is not strict with order
        score = False
        for pred in prediction:
            this_score = get_rru_score(pred)
            score = True if this_score else False
            if score:
                # If we have one hit it is enough
                break

    else:
        # JSON based or single prediction test
        score = get_rru_score(prediction)

    result["call_match"] = score

    return result


def build_predictions_instruct(
    resps: list[list[str]],
    docs: list[dict],
) -> list[list[str]]:
    return build_predictions_instruct_with_tools(
        resps, docs, [[None for _ in r] for r in resps]
    )


def build_predictions_instruct_with_tools(
    resps: list[list[str]],
    docs: list[dict],
    tool_calls: list[list[dict]],
    reasoning: list[list[str]] = None,
    **kwargs,
) -> list[list[str]]:
    """
    Parse and structure raw instruction-following responses for evaluation.

    Processes raw model responses by parsing them according to the specified format
    (JSON, string template or tool_calls). Extracts thought, action, and arguments.
    On parse errors, returns the raw response with empty field mappings.

    Args:
        resps (list[list[str]]): Nested list of model responses, one list per document
        docs (list[dict]): Document metadata with keys:
            - 'response_format': Format type ('json' or 'str')
            - 'evaluation_data': JSON template for parsing
        tool_calls (list[list[dict]]): Nested list of model tool calls, one list per document
        reasoning (list[list[str]]): Nested list of model reasoning traces, one list per document

    Returns:
        list[list[dict]]: Structured predictions with each dict containing:
            - 'response': Parsed response dict
            - 'test_keys_dict': Mapping of template field names
            - 'call_dict': Mapping for action/args extraction
    """
    if reasoning is None:
        # Patch reasoning trace
        reasoning = [[None for _ in r] for r in resps]
    assert (
        len(resps) == len(docs)
    ), "Number of response instances not matching number of documents, cannot procede to eval."
    assert (
        len(resps) == len(tool_calls)
    ), "Number of response instances not matching number of tool call instances, cannot procede to eval."
    assert (
        len(resps) == len(reasoning)
    ), "Number of response instances not matching number of reasoning instances, cannot procede to eval."
    out = list()
    for resp, doc, tool_call, reason in zip(resps, docs, tool_calls, reasoning):
        assert (
            len(resp) == len(tool_call)
        ), "Number of responses not matching number of tool calls, cannot procede to eval."
        assert (
            len(resp) == len(reason)
        ), "Number of responses not matching number of reasoning traces, cannot procede to eval."
        response_format = doc["response_format"]
        template = json.loads(doc["evaluation_data"])
        out.append(list())
        for r, t, y in zip(resp, tool_call, reason):
            try:
                parsed_resp, test_keys_dict, call_dict = process_response_instruct(
                    r, t, y, response_format, template
                )
                # Add to outputs
                out[-1].append(
                    {
                        "response": parsed_resp,
                        "test_keys_dict": test_keys_dict,
                        "call_dict": call_dict,
                    }
                )
            except Exception as e:
                print(str(e))
                # Add failed parse
                out[-1].append(
                    {
                        "failed_parse": r,
                        "expected_format": response_format,
                    }
                )

    return out


def build_predictions_call(resps: list[list[str]], docs: list[dict]) -> list[list[str]]:
    return build_predictions_call_with_tools(
        resps, docs, [[None for _ in r] for r in resps]
    )


def build_predictions_call_with_tools(
    resps: list[list[str]],
    docs: list[dict],
    tool_calls: list[list[dict]],
    reasoning: list[list[str]] = None,
    **kwargs,
) -> list[list[str]]:
    """
    Parse and structure raw function call responses for evaluation.

    Processes raw model responses by parsing them into structured function calls
    in the specified format (currently only JSON supported). On parse errors,
    returns error info with the failed parse data and expected format.

    Args:
        resps (list[list[str]]): Nested list of model responses, one list per document
        docs (list[dict]): Document metadata with keys:
            - 'response_format': Format type ('json' or other)
        tool_calls (list[list[dict]]): Nested list of model tool calls, one list per document
        reasoning (list[list[str]]): Nested list of model reasoning traces, one list per document

    Returns:
        list[list[dict|list]]: Parsed predictions, each containing:
            - On success: dict with 'name' and 'args' fields
            - On failure: list with dict containing 'failed_parse' and 'expected_format'
    """
    if reasoning is None:
        # Patch reasoning trace
        reasoning = [[None for _ in r] for r in resps]

    assert (
        len(resps) == len(docs)
    ), "Number of response instances not matching number of documents, cannot procede to eval."
    assert (
        len(resps) == len(tool_calls)
    ), "Number of response instances not matching number of tool call instances, cannot procede to eval."
    assert (
        len(resps) == len(reasoning)
    ), "Number of response instances not matching number of reasoning instances, cannot procede to eval."
    out = list()
    for resp, doc, tool_call, reason in zip(resps, docs, tool_calls, reasoning):
        assert (
            len(resp) == len(tool_call)
        ), "Number of responses not matching number of tool calls, cannot procede to eval."
        assert (
            len(resp) == len(reason)
        ), "Number of responses not matching number of reasoning traces, cannot procede to eval."
        response_format = doc["response_format"]
        out.append(list())
        for r, t, y in zip(resp, tool_call, reason):
            try:
                parsed_resp = process_generation(r, t, y, response_format)
            except Exception as e:
                print(str(e))
                parsed_resp = [{"failed_parse": r, "expected_format": response_format}]
            out[-1].append(parsed_resp)

    return out


################################################################################
#      T-Eval Functions Adapted from https://github.com/open-compass/T-Eval
################################################################################

# ------------------------------------------------------------------------------
#                          Test Sp[ecific Functions
# ------------------------------------------------------------------------------


def construct_review_response_groups(target):
    """
    Construct A-VERT candidate groups for review/ranking tasks.

    Separates multiple-choice options (A-H) into correct and wrong groups, then
    builds A-VERT candidate groups with semantic embeddings for comparison.

    Args:
        target (str): The correct option letter (A-H)

    Returns:
        dict: A-VERT group texts dict with 'correct' and 'wrong' keys containing
            embeddings and candidate options
    """
    # Possible targets in dataset
    choices = [
        "A",
        "B",
        "C",
        "D",
        "E",
        "F",
        "G",
        "H",
    ]

    def get_review_options(target, choices):
        """Separate correct option from wrong options."""
        correct_group_text = list()
        wrong_group_text = list()
        correct_group_idxs = list()
        wrong_group_idxs = list()
        for idx in range(len(choices)):
            if choices[idx] == target:
                correct_group_text.append(choices[idx])
                correct_group_idxs.append(idx)
            else:
                wrong_group_text.append(choices[idx])
                wrong_group_idxs.append(idx)

        return (
            correct_group_text,
            wrong_group_text,
            correct_group_idxs,
            wrong_group_idxs,
        )

    correct_group_text, wrong_group_text, correct_group_idxs, wrong_group_idxs = (
        get_review_options(target, choices)
    )

    # Construct the wrong candidates group
    group_texts_dict = a_vert.processing.construct_candidate_groups(
        correct_group_text,
        wrong_group_text,
        ["correct", "wrong"],
        enhance=ENHANCE,
        with_options=ENHANCE,
        option_symbol="letters",
        correct_group_idxs=correct_group_idxs,
        wrong_group_idxs=wrong_group_idxs,
    )

    return group_texts_dict


def process_response_instruct(response, tool_call, reason, response_format, template):
    """
    Parse instruction-following response into structured components.

    Extracts thought, action, and arguments from model response based on format.
    For JSON and tool_calls formats: remaps keys according to template. For string format: parses
    using template delimiters and converts args to JSON.

    Args:
        response (str): Raw model response
        tool_call (dict): Model tool calls
        reason (str): Model reasoning trace (not used, TODO: handle it as "thought" ?)
        response_format (str): Format type - 'json' or 'str'
        template (dict): Template configuration:
            - For 'json': maps expected keys to response keys
            - For 'str': contains start/end delimiters (thought_start, thought_end, etc.)

    Returns:
        tuple: (parsed_response, test_keys_dict, call_dict) where:
            - parsed_response (dict): Extracted components
            - test_keys_dict (dict): Mapping of template keys for evaluation
            - call_dict (dict): Mapping for function name and args extraction

    Raises:
        ValueError: If response_format is not 'json' or 'str'
    """

    def patch_keys(resp, template):
        for key in template.keys():
            if template[key] != key:
                if template[key] in resp.keys():
                    resp[key] = resp[template[key]]
                    resp.pop(template[key], None)
                else:
                    # fill with none, it might not even be necessary later during evaluation
                    resp[key] = None
        return resp

    if response_format == "json":
        # Parse the response
        resp = format_load(response)
        # Apply the key conversion for evaluation
        resp = patch_keys(resp, template)
        # Assign
        test_keys_dict = template
        call_dict = {
            "name": "action",
            "args": "args",
        }

    elif response_format == "tool_calls":
        # The tool should be already the one requested in the correct format
        # placed as the first tool to be called in the "function" field
        resp = tool_call[0]["function"]
        # Note, it is OK for this to fail if the model do not request a tool
        # we will handle this later and fail the task.

        # Apply the key conversion for evaluation
        resp = patch_keys(resp, template)
        # Assign
        test_keys_dict = template
        call_dict = {
            "name": "action",
            "args": "args",
        }

    elif response_format == "str":
        thought_start = template["thought_start"]
        thought_end = template["thought_end"]
        action_start = template["action_start"]
        action_end = template["action_end"]
        args_start = template["args_start"]
        args_end = template["args_end"]
        parse_template = (
            thought_start
            + "{thought}"
            + thought_end
            + action_start
            + "{action}"
            + action_end
            + args_start
            + "{args}"
            + args_end
        )
        resp = parse_string(parse_template, response, allow_newline=True)
        # Patch json args
        resp["args"] = json.loads(resp["args"].replace("'", '"'))
        resp["action"] = resp["action"].strip()

        test_keys_dict = {
            "thought": "thought",
            "action": "action",
            "args": "args",
        }
        call_dict = {
            "name": "action",
            "args": "args",
        }

    else:
        raise ValueError(f"Response format {response_format} not supported.")

    return resp, test_keys_dict, call_dict


def instruct_scorer(template, call_dict, gt_parsed, resp_parsed):
    """
    Score instruction-following response by matching all tested components.

    Compares predicted response against ground truth using: function call matching
    (action + args) and field-by-field comparison for non-action fields. Returns
    binary match (0 or 1) as product of all component matches.

    Args:
        template (dict): Keys of fields to test (e.g., 'thought', 'action', 'args')
        call_dict (dict): Mapping for action/args extractionf
        gt_parsed (dict): Ground truth parsed response
        resp_parsed (dict): Predicted parsed response

    Returns:
        int: Binary score (0 or 1) indicating if all tested components match

    Raises:
        ValueError: If no fields are tested
    """
    is_match = True
    call_tested = False
    one_test = False
    for key in template.keys():
        if (key == "action" or key == "args") and not call_tested:
            is_match *= call_matching(
                resp_parsed, gt_parsed, strict_numerical_match=True, call_dict=call_dict
            )
            call_tested = True
            one_test = True
        elif (key != "action" and key != "args") and (key in gt_parsed.keys()):
            is_match *= gt_parsed[key] == resp_parsed[template[key]]
            one_test = True
    if not one_test:
        print(
            f"WARNING: Sample not tested! Check definition! {resp_parsed} != {gt_parsed} ({template})"
        )
        is_match = False
    return is_match


def planing_scorer(pred_plan, gt_plan, strict_numerical_match=True) -> dict:
    """
    Calculate precision, recall, and F1 score between predicted and ground truth plans.

    Matches action sequences using: 1) Pairwise call matching to build scoring matrix,
    2) Hungarian algorithm for optimal bipartite matching, 3) Longest Increasing
    Subsequence (LIS) to count order-preserving matches.

    Args:
        pred_plan (list): Predicted actions, each with 'name', 'args', 'id' fields
        gt_plan (list): Ground truth actions, each with 'name', 'args', 'id' fields
        strict_numerical_match (bool): If True, enforce exact numerical argument matching.
            If False, only match numerical argument types. Defaults to True.

    Returns:
        dict: Metrics with keys:
            - 'precision': Ratio of correctly matched predicted actions to total predicted
            - 'recall': Ratio of correctly matched ground truth actions to total ground truth
            - 'f1_score': Harmonic mean of precision and recall (0 if either is 0)
    """
    if len(pred_plan) == 0 or len(gt_plan) == 0:
        return {"precision": 0, "recall": 0, "f1_score": 0}

    pred_plan = deepcopy(sorted(pred_plan, key=lambda x: x["id"]))
    gt_plan = deepcopy(sorted(gt_plan, key=lambda x: x["id"]))

    # Add end action
    # Currently it is hard-code
    if pred_plan[-1]["name"] == "FinishAction":
        pred_plan = pred_plan[:-1]
    if gt_plan[-1]["name"] == "FinishAction":
        gt_plan = gt_plan[:-1]
    # The total counts of nodes and edges.
    len_pred = len(pred_plan)
    len_gt = len(gt_plan)

    if len_pred == 0:
        # Only a single action that was "FinishAction", nothing to evaluate
        return {"precision": 0, "recall": 0, "f1_score": 0}

    matching_score_matrix = np.zeros((len_pred, len_gt))
    name_pred, args_pred = [], []
    name_gt, args_gt = [], []
    for i in range(len_pred):
        name_pred.append(pred_plan[i]["name"])
        # args come, in the generation, as string format
        try:
            args_pred.append(json.loads(pred_plan[i]["args"].replace("'", '"')))
        except Exception as _:
            args_pred.append("")
    for i in range(len_gt):
        name_gt.append(gt_plan[i]["name"])
        args_gt.append(gt_plan[i]["args"])

    # Check all nodes and see if they are equal
    for i in range(len_pred):
        for j in range(len_gt):
            # Assign the action match
            call_pred = {"name": name_pred[i], "args": args_pred[i]}
            call_gt = {"name": name_gt[j], "args": args_gt[j]}
            matching_score_matrix[i][j] = call_matching(
                call_pred, call_gt, strict_numerical_match=strict_numerical_match
            )

    G = nx.Graph()
    for i in range(len_pred):
        for j in range(len_gt):
            if matching_score_matrix[i][j]:
                G.add_edge(i, str(j), weight=matching_score_matrix[i][j])
    max_weight_matching = nx.max_weight_matching(G)

    pred_to_gt_mapping = dict()
    for key in max_weight_matching:
        if isinstance(key[0], int):
            pred_to_gt_mapping[int(key[0])] = int(key[1])
        else:
            pred_to_gt_mapping[int(key[1])] = int(key[0])

    # If a prediction node does not match any golden answer node, we mark the node as -1.
    for i in range(len_pred):
        if i not in pred_to_gt_mapping:
            pred_to_gt_mapping[i] = -1
    # Calculate how many nodes are matched by Longest Increasing Subsequence (LIS)
    dp = np.ones(len_pred)
    for i in range(len_pred):
        for j in range(i):
            if pred_to_gt_mapping[i] == -1 or pred_to_gt_mapping[j] == -1:
                continue
            if pred_to_gt_mapping[i] > pred_to_gt_mapping[j]:
                dp[i] = max(dp[i], dp[j] + 1)
    correct_count = int(max(dp))

    recall, precision = correct_count / len(gt_plan), correct_count / len(pred_plan)
    f1_score = 2 * recall * precision / (recall + precision)
    result = {"precision": precision, "recall": recall, "f1_score": f1_score}
    return result


# ==============================================================================
#                           GENERAL HELPER FUNCTIONS
# ==============================================================================
# Arguments that should use semantic matching (e.g., via A-VERT) rather than
# strict equality matching, as they contain natural language content.
SEMANTIC_MATCH_ARGS = ["thought", "query"]


def check_strict_numerical_match(prompt_hist, gt_args):
    """
    Determine if strict numerical matching should be enforced.

    Checks if all numerical arguments in the ground truth appear in the prompt
    history. If any numerical argument is missing from the prompt, strict matching
    is disabled (only type matching used instead).

    Args:
        prompt_hist (list): Conversation history where each item has 'content' field
        gt_args (dict): Ground truth function arguments mapping

    Returns:
        bool: True if all numerical arguments appear in prompt history, False otherwise
    """
    strict_num_match = False
    if len(gt_args) > 0 and isinstance(gt_args, dict):
        is_present = 0
        for argument in gt_args.keys():
            if isinstance(gt_args[argument], numbers.Number):
                is_present += check_arg_number_existence(prompt_hist, gt_args[argument])
        # We can be strict only if all numerical arguments are there
        strict_num_match = len(gt_args) == is_present
    return strict_num_match


def check_arg_number_existence(prompt_hist, number):
    """
    Check if a number appears anywhere in the prompt history.

    Args:
        prompt_hist (list): Conversation history to search
        number: Numerical value to find as string

    Returns:
        bool: True if number found in any prompt content, False otherwise
    """
    for prompt in prompt_hist:
        if str(number) in prompt["content"]:
            return True
    return False


def process_generation(
    pred_data,
    tool_call_data,
    reason_data,
    prompt_type,
) -> dict:
    """
    Parse and validate function call response in specified format.

    Converts raw response string to structured format. Currently only JSON format
    is fully supported.

    Args:
        pred_data (str): Raw model response
        prompt_type (str): Expected format type ('json', 'tool_calls', 'str', or 'ReWOO')
        tool_call_data (list[dict]): Tools called by the model
        reason_data (str): Model reasoning trace

    Returns:
        dict: Parsed function call data with 'name' and 'args' fields

    Raises:
        ValueError: If JSON parsing fails or unsupported/unimplemented format used
        NotImplementedError: If format other than 'json' or 'str' provided
    """
    if prompt_type == "json":
        pred_data = format_load(pred_data)
        if pred_data == []:
            raise ValueError("Unable to load JSON data")
    elif prompt_type == "tool_calls":
        if len(tool_call_data) > 0:
            # The model called tools
            pred_data = list()
            for idx, tool_call in enumerate(tool_call_data):
                # The tool should be already the one requested in the correct format
                this_call = tool_call["function"]
                # Patch the arguments field
                this_call["args"] = this_call["arguments"]
                this_call.pop("arguments", None)
                this_call["id"] = idx
                # Add the call
                pred_data.append(this_call)

            if len(pred_data) == 1:
                # This is a single tool call, colapse
                pred_data = pred_data[0]
        else:
            # The model did not call any tool, this case is a "FinishAction", so
            # lets fill that in
            pred_data = {
                "id": 0,
                "name": "FinishAction",
                "args": pred_data,  # The response that goes to the evaluator
                "thought": None,  # Filled in later
            }

    elif prompt_type == "ReWOO":
        raise ValueError("Deprecated type")
    elif prompt_type == "str":
        raise ValueError("Type STR not implemented yet (need to modify scoring too)")
    else:
        raise NotImplementedError(
            f"Currently, we only support json and str format, but get {prompt_type}"
        )

    # Add thought if not stated directly by the model
    if isinstance(pred_data, list):
        for this_pred_data in pred_data:
            if "thought" in this_pred_data.keys() and this_pred_data["thought"] is None:
                this_pred_data["thought"] = reason_data  # Model reasoning trace
    else:
        if "thought" in pred_data.keys() and pred_data["thought"] is None:
            pred_data["thought"] = reason_data  # Model reasoning trace

    return pred_data


def call_matching(
    call_pred,
    call_gt,
    strict_numerical_match=True,
    call_dict={"name": "name", "args": "args"},
) -> bool:
    """
    Compare predicted function call against ground truth call.

    Matches function name first, then validates all arguments. For each argument:
    - Semantic args (thought, query): Currently skipped (TODO: implement A-VERT)
    - Numerical args: Type-checked only if strict_numerical_match=False, else exact match
    - Other args: Exact string/value match

    Returns binary match (True/False).

    Args:
        call_pred (dict): Predicted call with structure based on call_dict
        call_gt (dict): Ground truth call with structure based on call_dict
        strict_numerical_match (bool): If True, require exact numerical values.
            If False, only check that values are numeric types. Defaults to True.
        call_dict (dict): Maps 'name' and 'args' keys to field names in call objects.
            Defaults to {'name': 'name', 'args': 'args'}

    Returns:
        bool: True if function name and all arguments match according to rules
    """
    is_match = False
    # Assign the action match
    is_match = call_pred[call_dict["name"]] == call_gt[call_dict["name"]]
    # If the action is matched, check the arguments
    if is_match:
        if isinstance(call_gt[call_dict["args"]], str):
            # Check if this is type "FinishAction", because there all fields are
            # semantic matches
            if call_gt[call_dict["name"]] == "FinishAction":
                # TODO: Implement A-VERT for semantic matching, but would need
                # accurate ground truth thought traces to implement properly
                pass
            else:
                # Strict match otherwise
                is_match *= call_gt[call_dict["args"]] == call_pred[call_dict["args"]]
        else:
            for argument in call_gt[call_dict["args"]].keys():
                # Ensure argument is a dict
                if not isinstance(call_pred[call_dict["args"]], dict):
                    try:
                        call_pred[call_dict["args"]] = json.loads(
                            call_pred[call_dict["args"]]
                        )
                    except Exception as _:
                        is_match = False
                # If still valid, check argument name
                if is_match:
                    is_match *= argument in call_pred[call_dict["args"]].keys()
                # Now, if still a match, check parameter given
                if is_match:
                    if argument in SEMANTIC_MATCH_ARGS:
                        # TODO: Implement A-VERT for semantic matching, but would need
                        # accurate ground truth thought traces to implement properly
                        pass

                    elif isinstance(
                        call_gt[call_dict["args"]][argument], numbers.Number
                    ):
                        # Numerical argument
                        if not strict_numerical_match:
                            # Just check the type
                            is_match *= isinstance(
                                call_pred[call_dict["args"]][argument], numbers.Number
                            )
                        else:
                            # strict match
                            is_match *= (
                                call_gt[call_dict["args"]][argument]
                                == call_pred[call_dict["args"]][argument]
                            )
                    else:
                        # Normal argument that will be strict-matched
                        is_match *= (
                            call_gt[call_dict["args"]][argument]
                            == call_pred[call_dict["args"]][argument]
                        )
                else:
                    break
    return is_match


def format_load(raw_data: str, start_character: str = "", end_character: str = ""):
    """
    Extract and parse JSON/Python data from raw text response.

    Handles common LLM response patterns: markdown code blocks (```json, ```),
    and optional character slicing. Attempts multiple parsing strategies:
    1) ast.literal_eval (Python syntax)
    2) json.loads (standard JSON)
    3) json.loads with quote normalization (single to double quotes)

    Args:
        raw_data (str): Raw text response from model
        start_character (str): If provided, slice string from first occurrence of this char
        end_character (str): If provided, slice string up to last occurrence of this char

    Returns:
        dict|list: Parsed data structure

    Raises:
        Exception: If data cannot be parsed with any strategy
    """
    if not isinstance(raw_data, str):
        # the data has been evaluated
        return raw_data
    if "```json" in raw_data:
        raw_data = raw_data[raw_data.find("```json") + len("```json") :]
        raw_data = raw_data.strip("`")
    elif "```" in raw_data:
        raw_data = raw_data[raw_data.find("```") + len("```") :]
        raw_data = raw_data.strip("`")
    if start_character != "":
        raw_data = raw_data[raw_data.find(start_character) :]
    if end_character != "":
        raw_data = raw_data[: raw_data.rfind(end_character) + len(end_character)]
    successful_parse = False
    try:
        data = ast.literal_eval(raw_data)
        successful_parse = True
    except Exception:
        pass
    if not successful_parse:
        try:
            raw_data = re.sub(r"//.*?\n", "", raw_data)
            data = json.loads(raw_data)
            successful_parse = True
        except Exception:
            pass
    if not successful_parse:
        try:
            data = json.loads(raw_data.replace("'", '"'))

            successful_parse = True
        except Exception:
            pass
    if not successful_parse:
        raise Exception("Cannot parse raw data")
    return data


def parse_string(
    template: str, input_string: str, allow_newline: bool = False
) -> dict | None:
    """
    Extract structured data from input string using template pattern.

    Uses Python string formatting to define a regex pattern, then matches input
    against it. Handles duplicate keys by collecting values into lists.

    Args:
        template (str): Format string with placeholders, e.g., '{who} like {what}'.
            Each {key} becomes a capture group in regex.
        input_string (str): String to parse against template
        allow_newline (bool): If True, capture groups can match across newlines.
            Uses re.S flag if True. Defaults to False.

    Returns:
        dict: Mapping of template keys to captured values from input_string.
            For duplicate keys, values collected into a list.
            Returns None if input_string doesn't match template pattern.

    Examples:
        >>> parse_string('{who} like {what}', 'monkey like banana')
        {'who': 'monkey', 'what': 'banana'}

        >>> parse_string('{who} like {what}', 'monkey likes banana')
        None

        >>> parse_string('{animal} like {food} and {food}', 'monkey like banana and apple')
        {'animal': 'monkey', 'food': ['banana', 'apple']}
    """
    formatter = Formatter()
    context = []
    keys = []
    for v in formatter.parse(template):
        # v is (literal_text, field_name, format_spec, conversion)
        if v[1] is not None:
            keys.append(v[1])
        context.append(v[0])
    pattern = template
    for k in keys:
        pattern = pattern.replace("{" + f"{k}" + "}", "(.*)")
    # pattern = re.compile(rf'{pattern}')
    values = re.findall(pattern, input_string, re.S if allow_newline else 0)
    if len(values) < 1:
        return None
    data = dict()
    for k, v in zip(keys, values[0]):
        if k in data:
            tmp = data[k]
            if isinstance(tmp, list):
                data[k].append(v)
            else:
                data[k] = [tmp, v]
        else:
            data[k] = v
    return data
