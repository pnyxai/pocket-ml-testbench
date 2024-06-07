import json
import os
import shutil
home = os.environ['HOME']

from bson.objectid import ObjectId
from hashlib import sha256
from pathlib import Path
from transformers import AutoTokenizer, PreTrainedTokenizer, PreTrainedTokenizerFast
from typing import List, Optional, Union

from app.app import get_app_logger
eval_logger = get_app_logger("sample")

def _get_tokenizer_jsons(tokenizer: Union[PreTrainedTokenizer, PreTrainedTokenizerFast])-> dict:
    """Get tokenizer jsons been used"""
    CURRENT_DIR = os.path.dirname(__file__)
    EPHIMERAL_FOLDER_NAME = "tmp_tokenizer"
    TOKENIZER_EPHIMERAL_PATH = Path(
        os.path.join(CURRENT_DIR, EPHIMERAL_FOLDER_NAME))

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
        raise RuntimeError(
            f"Error removing '{TOKENIZER_EPHIMERAL_PATH.name}' dir: {e}"
        ) from e

    return tokenizer_jsons

def prepare_tokenizer(tokenizer: Union[PreTrainedTokenizer, PreTrainedTokenizerFast])-> dict:
     
    tokenizer_jsons = _get_tokenizer_jsons(tokenizer)


    if 'model_max_length' in tokenizer_jsons['tokenizer_config']:
        tokenizer_jsons['tokenizer_config']['model_max_length'] = str(tokenizer_jsons['tokenizer_config']['model_max_length'])


    hash = json.dumps(tokenizer_jsons, sort_keys=True).encode('utf-8')
    tokenizer_hash = sha256(hash).hexdigest()
    tokenizer_mongo = {'tokenizer': tokenizer_jsons, 'hash': tokenizer_hash, '_id': ObjectId()}
    return tokenizer_mongo

def load_tokenizer(tokenizer_objects: dict, wf_id:str, tokenizer_ephimeral_path: str=None)-> Union[PreTrainedTokenizer, PreTrainedTokenizerFast]:

    if tokenizer_ephimeral_path is None:
        tokenizer_ephimeral_path = Path(os.path.join(home, 'tokenizer_ephimeral', wf_id))
    else:
        tokenizer_ephimeral_path = Path(tokenizer_ephimeral_path)
    tokenizer_ephimeral_path.mkdir(parents=True, exist_ok=True)

    for key, value in tokenizer_objects.items():
        with open(
            os.path.join(tokenizer_ephimeral_path, key + ".json"), "w"
        ) as f:
            json.dump(value, f)
            f.close()
    
    tokenizer = AutoTokenizer.from_pretrained(
        tokenizer_ephimeral_path
    )
    try:
        shutil.rmtree(tokenizer_ephimeral_path)
        eval_logger.debug(
            f"Ephimeral '{tokenizer_ephimeral_path.name}' directory removed successfully."
        )
        eval_logger.debug(
            f"Tokenizer objects availables: {str(tokenizer_objects.keys())}"
        )
    except OSError as e:
        raise RuntimeError(
            f"Error removing '{tokenizer_ephimeral_path.name}' directory: {e}"
        )
    return tokenizer