from temporalio import activity
import random
from app.app import get_app_logger


async def do_a_sum(a: int) -> int:
    return random.randint(a=a, b=a * 2) * 2


@activity.defn
async def random_int(a: int) -> int:
    eval_logger = get_app_logger("sample")
    # multiply by 2 a random int
    r = 0
    for i in range(a):
        r += await do_a_sum(i)

    eval_logger.debug(f'Result: {r}')
    return r
