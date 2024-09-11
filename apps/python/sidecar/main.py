from app.app import get_app_logger, setup_app
from app.config import read_config
from fastapi import FastAPI
from fastapi.responses import JSONResponse
from transformers import AutoConfig, AutoTokenizer

from packages.python.lmeh.utils.tokenizers import prepare_config, prepare_tokenizer

###################################################
# SET UP SIDECAR
###################################################
cfg = read_config()

app_config = setup_app(cfg)

config = app_config["config"]

logger = get_app_logger("sidecar")
logger.info("starting sidecar")
TOKENIZER_EPHIMERAL_PATH = "/tmp/tokenizer_aux"
CONFIG_EPHIMERAL_PATH = "/tmp/config_aux"
# Read tokenizer data
tokenizer = AutoTokenizer.from_pretrained(config["tokenizer_path"])
# Process it using the MLTB library (functions are reused by the MLTB)
TOKENIZER_JSON, TOKENIZER_HASH = prepare_tokenizer(
    tokenizer, TOKENIZER_EPHIMERAL_PATH=TOKENIZER_EPHIMERAL_PATH
)

_config = AutoConfig.from_pretrained(config["tokenizer_path"])
CONFIG_JSON, CONFIG_HASH = prepare_config(
    _config, CONFIG_EPHIMERAL_PATH=CONFIG_EPHIMERAL_PATH
)

# add config to tokenizer json
TOKENIZER_JSON.update(CONFIG_JSON)


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
    logger.debug("returning tokenizer data")
    return JSONResponse(content=TOKENIZER_JSON)


# -----------------------------------------------
# Get Tokenizer Hash
# -----------------------------------------------
@app.get("/pokt/tokenizer-hash")
def get_tokenizer_hash():
    logger.debug("returning tokenizer hash")
    return JSONResponse({"hash": TOKENIZER_HASH})


# -----------------------------------------------
# Get Full Config
# -----------------------------------------------
@app.get("/pokt/config")
def get_config():
    logger.debug("returning config data")
    return JSONResponse(content=CONFIG_JSON)


# -----------------------------------------------
# Get Config Hash
# -----------------------------------------------
@app.get("/pokt/config-hash")
def get_config_hash():
    logger.debug("returning config hash")
    return JSONResponse({"hash": CONFIG_HASH})
