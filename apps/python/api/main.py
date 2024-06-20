import os

from app.leaderboard import get_leaderboard_full
from app.basemodels import PoktMongodb
from fastapi import Depends, FastAPI, HTTPException, Request
from app.logger import init_logger
from fastapi.responses import JSONResponse
from fastapi.middleware.cors import CORSMiddleware

logger = init_logger(__name__)

# Authorization header # TODO
# WEB_AUTH_TOKEN = os.getenv("WEB_AUTH_TOKEN") 

###################################################
# MONGO DB
###################################################
MONGO_URI = os.getenv("MONGODB_URI", None)
if MONGO_URI == None:
    raise ValueError("MONGODB_URI not set")

###################################################
# GLOBALS
###################################################

# # Authorization check
# def web_auth_func(request: Request):
#     if request.headers.get("Authorization", None) != WEB_AUTH_TOKEN:
#         raise HTTPException(status_code=401, detail="Forbidden - WEB")


# Connect to mongo
mongo_db = PoktMongodb(MONGO_URI)

# Create serving app
app = FastAPI()

origins = ["*"]

app.add_middleware(
    CORSMiddleware,
    allow_origins=origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

###################################################
# ENDPOINTS
###################################################

# -----------------------------------------------
# Test Endpoint - Return Leaderboard
# -----------------------------------------------

# async def get_leaderboard(input, dependency: str = Depends(web_auth_func)):
@app.get("/leaderboard")
async def get_leaderboard():

    # Connect to mongo
    mongo_db = PoktMongodb(MONGO_URI)
    # Check
    await mongo_db.ping()
    # Get
    response, success = await get_leaderboard_full(mongo_db)

    if not success:
        raise HTTPException(status_code=406, detail="Cannot retrieve leaderboard data.")

    return JSONResponse(response)
