from transformers import AutoConfig, AutoTokenizer, PretrainedConfig

from packages.python.lmeh.utils.tokenizers import prepare_config, prepare_tokenizer
import os
from app.overrides import do_request

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


async def setup_llm_backend_override(endpoint_override_data, logger):
    """
    Reads the backend endpoint data, sets-up the URI and checks the health.
    """

    LLM_BACKEND_ENDPOINT = None
    LLM_BACKEND_MODEL_NAME = None

    if endpoint_override_data is None:
        logger.info("LLM backend overriding not configured.")
    else:
        LLM_BACKEND_ENDPOINT = endpoint_override_data["backend_path"]
        LLM_BACKEND_MODEL_NAME = endpoint_override_data["backend_model_name"]

        # Test both endpoints
        _ = await do_request(
            LLM_BACKEND_ENDPOINT,
            "/v1/completions",
            {"prompt": "123456", "max_tokens": 2, "model": LLM_BACKEND_MODEL_NAME},
            error_on_not_OK=True,
            logger=logger,
        )
        _ = await do_request(
            LLM_BACKEND_ENDPOINT,
            "/v1/chat/completions",
            {
                "messages": [
                    {"role": "system", "content": "You are prime number 7."},
                    {"role": "user", "content": "who is your next relative?"},
                ],
                "max_tokens": 2,
                "model": LLM_BACKEND_MODEL_NAME,
            },
            error_on_not_OK=True,
            logger=logger,
        )

        logger.info("Backend healthy!")

    return LLM_BACKEND_ENDPOINT, LLM_BACKEND_MODEL_NAME
