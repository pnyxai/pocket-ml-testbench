import aiohttp
from typing import Dict, Any
import os


def replace_model_name(json_request, override):
    if json_request.get("model", None) is None:
        return None
    else:
        json_request["model"] = override

    return json_request


async def do_request(
    url: str,
    path: str,
    payload: Dict[str, Any],
    headers: Dict[str, str] = None,
    timeout: int = 600,
    error_on_not_OK: bool = False,
    logger: Any = None,
) -> Dict[str, Any]:
    # Default headers
    default_headers = {"Content-Type": "application/json", "Accept": "application/json"}
    # In case backend is gated
    access_token = os.getenv("BACKEND_TOKEN", None)
    if access_token is not None:
        default_headers["Authorization"] = access_token

    # Merge default headers with custom headers
    request_headers = {**default_headers, **(headers or {})}
    timeout = aiohttp.ClientTimeout(total=timeout, sock_read=timeout)
    async with aiohttp.ClientSession(timeout=timeout) as session:
        try:
            async with session.post(
                url + path,
                headers=request_headers,
                json=payload,
                # Optional configurations
                timeout=timeout,  # 30 seconds timeout
                # ssl=True    # Enable SSL verification
            ) as response:
                # Raise an exception for bad status codes
                if not response.ok:
                    error_text = await response.text()
                    if logger is not None:
                        logger.debug(
                            "Request NOT OK",
                            status=response.status,
                            url=url,
                            error_text=error_text,
                        )
                    if error_on_not_OK:
                        response.raise_for_status()
                return await response.json()

        except aiohttp.ClientError as e:
            print(f"Request failed: {str(e)}")
            raise
