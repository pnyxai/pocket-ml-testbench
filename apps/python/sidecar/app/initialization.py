from transformers import AutoConfig, AutoTokenizer, PretrainedConfig

from packages.python.lmeh.utils.tokenizers import prepare_config, prepare_tokenizer
import requests
import json
import os

TOKENIZER_EPHIMERAL_PATH = "/tmp/tokenizer_aux"
CONFIG_EPHIMERAL_PATH = "/tmp/config_aux"


def setup_tokenizer_data(tokenizer_path_or_name):
    """
    Reads a tokenizer file from a given model folder or downloads an specified
    tokenizer from the Huggingface hub.
    It also calculates the tokenizer data hash.
    """

    # Read tokenizer data
    tokenizer = AutoTokenizer.from_pretrained(
        tokenizer_path_or_name, token=os.getenv("HF_TOKEN", None)
    )
    # Process it using the MLTB library (functions are reused by the MLTB)
    TOKENIZER_JSON, TOKENIZER_HASH = prepare_tokenizer(
        tokenizer, TOKENIZER_EPHIMERAL_PATH=TOKENIZER_EPHIMERAL_PATH
    )

    return TOKENIZER_JSON, TOKENIZER_HASH


def setup_model_config_data(config_path, config_data):
    """
    Reads a configuration file from a given model folder or creates an empty one
    using the provided config data.
    The empty configuration is filled with minimal data required by users:
    - model_public_name : A name that will be public, can be any string.
    - max_position_embeddings : The total number of tokens accepted by the model (input + output).

    It also calculates the resulting config data hash.
    """

    if config_path is not None and config_data is not None:
        raise ValueError(
            'Both "config_path" and "config_data" cannot be defined. Please define only one in the config file.'
        )

    elif config_path is not None:
        _config = AutoConfig.from_pretrained(config_path)

    elif config_data is not None:
        _config = PretrainedConfig(
            model_name=config_data["model_public_name"],
            max_position_embeddings=config_data["max_position_embeddings"],
            pokt_network_custom=True,
        )

    else:
        raise ValueError(
            'Both "config_path" and "config_data" cannot be empty. Please define one in the config file.'
        )

    CONFIG_JSON, CONFIG_HASH = prepare_config(
        _config, CONFIG_EPHIMERAL_PATH="./outputs/test"
    )

    return CONFIG_JSON, CONFIG_HASH


def setup_llm_backend_override(endpoint_override_data):
    """
    Reads the backend endpoint data, sets-up the URI and checks the health.
    """

    LLM_BACKEND_ENDPOINT = None
    LLM_BACKEND_MODEL_NAME = None

    if endpoint_override_data is None:
        print("LLM backend overriding not configured.")
    else:
        LLM_BACKEND_ENDPOINT = endpoint_override_data["backend_path"]
        LLM_BACKEND_MODEL_NAME = endpoint_override_data["backend_model_name"]

        # Check LLM backend health, should get a 200
        default_headers = {
            "Content-Type": "application/json",
            "Authorization": os.getenv("BACKEND_TOKEN"),  # In case backend is gated
        }
        req = requests.post(
            f"{LLM_BACKEND_ENDPOINT}/v1/completions",
            headers=default_headers,
            # auth=auth, TODO : Implement auths, for openAI and such
            data=json.dumps(
                {"prompt": "123456", "max_tokens": 2, "model": LLM_BACKEND_MODEL_NAME}
            ),
        )

        if req.status_code == 200:
            print("Backend healthy!")
        else:
            raise ValueError(
                f'Testing the "/v1/completions" endpoint resulted in a non 200 status code:\nstatus: {req.status_code}\nresponse: {req.json}'
            )

    return LLM_BACKEND_ENDPOINT, LLM_BACKEND_MODEL_NAME
