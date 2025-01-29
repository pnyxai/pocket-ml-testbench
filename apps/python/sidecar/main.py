from app.app import get_app_logger, setup_app
from app.config import read_config
from app.overrides import replace_model_name, do_request
from app.initialization import (
    setup_tokenizer_data,
    setup_model_config_data,
    setup_llm_backend_override,
)
from fastapi import FastAPI, Request, HTTPException
from fastapi.responses import JSONResponse

###################################################
# SET UP SIDECAR
###################################################
cfg = read_config()

app_config = setup_app(cfg)

config = app_config["config"]

logger = get_app_logger("sidecar")
logger.info("starting sidecar")


# Create the startup functions to load these variables
LLM_BACKEND_ENDPOINT, LLM_BACKEND_MODEL_NAME = None, None
TOKENIZER_JSON, TOKENIZER_HASH = None, None
CONFIG_JSON, CONFIG_HASH = None, None


async def startup_event():
    # Get variables
    global \
        LLM_BACKEND_ENDPOINT, \
        LLM_BACKEND_MODEL_NAME, \
        TOKENIZER_JSON, \
        TOKENIZER_HASH, \
        CONFIG_JSON, \
        CONFIG_HASH

    # Load tokenizer data
    TOKENIZER_JSON, TOKENIZER_HASH = setup_tokenizer_data(
        config["tokenizer_path_or_name"]
    )

    # Load or create model config data
    CONFIG_JSON, CONFIG_HASH = setup_model_config_data(
        config.get("model_config_path", None), config.get("model_config_data", None)
    )

    # Get overriding name, the actual name of the deployed model
    LLM_BACKEND_ENDPOINT, LLM_BACKEND_MODEL_NAME = await setup_llm_backend_override(
        config.get("llm_endpoint_override", None), logger
    )

    logger.debug("Setup done.")


# Create serving app
app = FastAPI()

# Add startup event handler
app.add_event_handler("startup", startup_event)

###################################################
# DATA ENDPOINTS
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


###################################################
# OVERRIDING ENDPOINTS
###################################################
@app.post("/v1/completions")
async def override_v1_completitions(request: Request):
    logger.debug("overriding /v1/completions")
    if LLM_BACKEND_ENDPOINT is None:
        raise HTTPException(
            status_code=501, detail="Backend LLM overriding is not configured."
        )

    try:
        # load
        data = await request.json()

        # replace model name
        data = replace_model_name(data, LLM_BACKEND_MODEL_NAME)
    except Exception:
        raise HTTPException(
            status_code=400, detail="Cannot interpret incoming request."
        )

    # call model endpoint
    response = await do_request(
        LLM_BACKEND_ENDPOINT, "/v1/completions", data, logger=logger
    )
    # return
    return JSONResponse(response)


@app.post("/v1/chat/completions")
async def override_v1_chat_completitions(request: Request):
    logger.debug("overriding /v1/chat/completions")
    if LLM_BACKEND_ENDPOINT is None:
        raise HTTPException(
            status_code=501, detail="Backend LLM overriding is not configured."
        )

    try:
        # load
        data = await request.json()

        # replace model name
        data = replace_model_name(data, LLM_BACKEND_MODEL_NAME)
    except Exception:
        raise HTTPException(
            status_code=400, detail="Cannot interpret incoming request."
        )

    # call model endpoint
    response = await do_request(
        LLM_BACKEND_ENDPOINT, "/v1/chat/completions", data, logger=logger
    )
    # return
    return JSONResponse(response)
