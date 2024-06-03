import os
import time

from app.app import get_app_logger, setup_app
from app.config import read_config
from fastapi import FastAPI
from tokenizers_utils.load import prepare_tokenizer_data

###################################################
# SET UP SIDECAR
###################################################
cfg = read_config()

app_config = setup_app(cfg)

config = app_config["config"]

l = get_app_logger("sidecar")
l.info("starting sidecar")

# Read tokenizer data
TOKENIZER_JSON, TOKENIZER_HASH = prepare_tokenizer_data(config["tokenizer_path"])


# Create serving app
app = FastAPI()

###################################################
# ENDPOINTS
###################################################

# -----------------------------------------------
# Get Full Tokenizer
# -----------------------------------------------
@app.get("/pokt-v1/tokenizer")
def get_tokenizer():
    l.debug("returning tokenizer data")
    return TOKENIZER_JSON


# -----------------------------------------------
# Get Tokenizer Hash
# -----------------------------------------------
@app.get("/pokt-v1/tokenizer-hash")
def get_tokenizer():
    l.debug("returning tokenizer hash")
    return TOKENIZER_HASH
