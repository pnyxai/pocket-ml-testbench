import asyncio
from temporalio import activity
from functools import wraps
from typing import Any, Awaitable, Callable, TypeVar, cast
from app.app import get_app_logger

F = TypeVar("F", bound=Callable[..., Awaitable[Any]])


def auto_heartbeater(fn: F) -> F:
    # We want to ensure that the type hints from the original callable are
    # available via our wrapper, so we use the functools wraps decorator
    @wraps(fn)
    async def wrapper(*args, **kwargs):
        event_logger = get_app_logger("auto_heartbeat")
        heartbeat_timeout = activity.info().heartbeat_timeout
        is_local = activity.info().is_local
        heartbeat_task = None
        if not is_local and heartbeat_timeout:
            event_logger.debug("heartbeat timeout is defined", heartbeat_timeout=heartbeat_timeout)
            # Heartbeat twice as often as the timeout
            heartbeat_task = asyncio.create_task(
                heartbeat_every(heartbeat_timeout.total_seconds() / 4)
            )
        else:
            event_logger.debug("no heartbeat timeout")

        try:
            return await fn(*args, **kwargs)
        finally:
            if heartbeat_task:
                event_logger.debug("heartbeat task finished")
                heartbeat_task.cancel()
                # Wait for heartbeat cancellation to complete
                await asyncio.wait([heartbeat_task])

    return cast(F, wrapper)


async def heartbeat_every(delay: float, *details: Any) -> None:
    # Heartbeat every so often while not canceled
    while True:
        activity_info = activity.info()
        event_logger = get_app_logger("auto_heartbeat")
        event_logger.debug(f"auto heartbeating", activity=activity_info.activity_id)
        activity.heartbeat(*details)
        await asyncio.sleep(delay)
