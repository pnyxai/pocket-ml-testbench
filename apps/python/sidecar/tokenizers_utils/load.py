import json
import os
import shutil
from hashlib import sha256
from pathlib import Path
from typing import Union

from app.app import get_app_logger
from transformers import AutoTokenizer, PreTrainedTokenizer, PreTrainedTokenizerFast

eval_logger = get_app_logger("sample")


def _get_tokenizer_jsons(tokenizer: Union[PreTrainedTokenizer, PreTrainedTokenizerFast]) -> dict:
    """Get tokenizer jsons been used"""
    CURRENT_DIR = os.path.dirname(__file__)
    EPHIMERAL_FOLDER_NAME = "tmp_tokenizer"
    TOKENIZER_EPHIMERAL_PATH = Path(os.path.join(CURRENT_DIR, EPHIMERAL_FOLDER_NAME))

    # save tokenizer files in ephimeral folder
    tokenizer.save_pretrained(TOKENIZER_EPHIMERAL_PATH.absolute())
    tmp_list = [i for i in TOKENIZER_EPHIMERAL_PATH.glob("*.json")]

    # populate tokenizer json
    tokenizer_jsons = {}
    for json_path in tmp_list:
        with open(json_path) as json_file:
            filename = json_path.stem
            tokenizer_jsons[filename] = json.load(json_file)
    try:
        shutil.rmtree(TOKENIZER_EPHIMERAL_PATH)
    except OSError as e:
        raise RuntimeError(f"Error removing '{TOKENIZER_EPHIMERAL_PATH.name}' dir: {e}") from e

    return tokenizer_jsons


def prepare_tokenizer(tokenizer: Union[PreTrainedTokenizer, PreTrainedTokenizerFast]) -> tuple[dict, str]:

    tokenizer_jsons = _get_tokenizer_jsons(tokenizer)

    if "model_max_length" in tokenizer_jsons["tokenizer_config"]:
        tokenizer_jsons["tokenizer_config"]["model_max_length"] = str(
            tokenizer_jsons["tokenizer_config"]["model_max_length"]
        )

    hash = json.dumps(tokenizer_jsons, sort_keys=True).encode("utf-8")
    tokenizer_hash = sha256(hash).hexdigest()
    return tokenizer_jsons, tokenizer_hash


def prepare_tokenizer_data(tokenizer_path: str) -> tuple[dict, str]:

    # Read tokenizer
    tokenizer = AutoTokenizer.from_pretrained(tokenizer_path)
    # Convert to json and create hash
    return prepare_tokenizer(tokenizer)
