from app.app import get_app_logger, setup_app
from app.config import read_config
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from transformers import AutoTokenizer

from packages.python.lmeh.utils.tokenizers import prepare_tokenizer

###################################################
# SET UP SIDECAR
###################################################
cfg = read_config()

app_config = setup_app(cfg)

config = app_config["config"]

l = get_app_logger("sidecar")
l.info("starting sidecar")

# Read tokenizer data
tokenizer = AutoTokenizer.from_pretrained(config["tokenizer_path"])
# Process it using the MLTB library (functions are reused by the MLTB)
TOKENIZER_JSON, TOKENIZER_HASH = prepare_tokenizer(tokenizer, TOKENIZER_EPHIMERAL_PATH="/tmp/tokenizer_aux")


# Create serving app
app = FastAPI()

###################################################
# ENDPOINTS
###################################################

# -----------------------------------------------
# Get Full Tokenizer
# -----------------------------------------------
@app.get("/pokt/tokenizer")
def get_tokenizer():
    l.debug("returning tokenizer data")
    return JSONResponse(content=TOKENIZER_JSON)


# -----------------------------------------------
# Get Tokenizer Hash
# -----------------------------------------------
@app.get("/pokt/tokenizer-hash")
def get_tokenizer():
    l.debug("returning tokenizer hash")
    return JSONResponse({"hash": TOKENIZER_HASH})
