from temporalio import activity
from typing import List
from protocol.protocol import CumSumRequest
import random
from app.app import get_app_logger

@activity.defn
async def random_int(a:int) -> int:
    eval_logger = get_app_logger("sample")
    # multiply by 2 a random int
    x = random.randint(a=a,b=a*2)*2
    eval_logger.debug(f'Result:',x=x)
    return x