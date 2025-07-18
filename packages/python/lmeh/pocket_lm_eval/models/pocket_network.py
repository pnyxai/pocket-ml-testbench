# Adapted from lm_eval/models/openai_completions.py
import copy
import json
from functools import cached_property
from typing import (
    Dict,
    List,
    Literal,
    Optional,
    Tuple,
    Union,
)


try:
    #    import requests
    #    from aiohttp import ClientSession, ClientTimeout, TCPConnector
    from tenacity import RetryError, retry, stop_after_attempt, wait_exponential
    from tqdm import tqdm
except ModuleNotFoundError:
    pass
from app.app import get_app_logger
from lm_eval.api.instance import Instance
from lm_eval.models.api_models import (
    TemplateAPI,
    JsonChatStr,
    LogLikelihoodInputs,
    create_image_prompt,
)
from lm_eval.models.openai_completions import (
    LocalCompletionsAPI,
    LocalChatCompletion,
)
from lm_eval.models.utils import configure_pad_token
from lm_eval.models.utils import Collator
from importlib.util import find_spec
from temporalio.exceptions import ApplicationError
from tqdm import tqdm

from packages.python.common.mongodb import MongoClient

# get_dtype,
# pad_and_concat,
# stop_sequences_criteria,
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.lmeh.utils.tokenizers import load_config, load_tokenizer
from packages.python.protocol.protocol import (
    CompletionRequest,
    ChatCompletionRequest,
    RequesterArgs,
    CompletionResponse,
    ChatCompletionResponse,
)

eval_logger = get_app_logger("sample")
evaluation_logger = get_app_logger("evaluation")

INVALID_ANSWER = "[invalidanswer]"

GENERATION_MAX_LENGHT = 8192


# this fuction change its behavior in 0.4.3.
# Currently we will mantain the previous behavior to be compatible with vLLM.
# ref:
# https://github.com/EleutherAI/lm-evaluation-harness/pull/1779#issuecomment-2161323224
# &
# https://github.com/EleutherAI/lm-evaluation-harness/issues/1949


def get_result(response, ctxlen: int) -> Tuple[float, bool]:
    """Process results from OpenAI API response.

    :param response: dict
        OpenAI API Response
    :param ctxlen: int
        Length of context (so we can slice them away and only keep the predictions)
    :return:
        continuation_logprobs: np.array
            Log probabilities of continuation tokens
        is_greedy: bool
            whether argmax matches given continuation exactly
    """
    is_greedy = True
    logprobs = response.logprobs.token_logprobs
    continuation_logprobs = sum(logprobs[ctxlen:])

    for i in range(ctxlen, len(response.logprobs.token_logprobs)):
        token = response.logprobs.token_logprobs[i]
        top_tokens = response.logprobs.top_logprobs[i]
        top_token = max(top_tokens.keys(), key=lambda x: top_tokens[x])
        if top_token != token:
            is_greedy = False
            break

    return continuation_logprobs, is_greedy


class SamplerAPI(TemplateAPI):
    _DEFAULT_MAX_LENGTH = GENERATION_MAX_LENGHT

    def __init__(
        self,
        requester_args: RequesterArgs,
        mongo_client: MongoClient,
        wf_id: str,
        model: str = "pocket_network",
        base_url: str = None,
        # Loglikelihood tasks require a tokenizer to calculate context lengths,
        # however the requests can be sent as a string if the API doesn't support token inputs.
        # use tokenized_requests=False
        tokenizer_backend: Optional[
            Literal["huggingface", "None", "none"]
        ] = "huggingface",
        truncate: bool = False,
        # number of concurrent requests. More useful if not batching
        num_concurrent: int = 1,
        max_retries: int = 3,
        max_gen_toks: int = GENERATION_MAX_LENGHT,
        batch_size: Union[str, int] = 1,
        seed: int = 1234,
        max_length: Optional[int] = None,
        add_bos_token: bool = False,
        custom_prefix_token_id: int = None,
        # send the requests as tokens or strings
        tokenized_requests: bool = False,
        trust_remote_code: bool = False,
        revision: Optional[str] = "main",
        use_fast_tokenizer: bool = True,
        verify_certificate: bool = True,
        eos_string: str = None,
        # timeout in seconds
        timeout: int = 300,
        max_images: int = 1,
        **kwargs,
    ) -> None:
        # super().__init__()
        missing_packages = [
            pkg
            for pkg in ["aiohttp", "tqdm", "tenacity", "requests"]
            if find_spec(pkg) is None
        ]
        if missing_packages:
            raise ModuleNotFoundError(
                f"Attempted to use an API model, but the required packages {missing_packages} are not installed. "
                'Please install these via `pip install lm-eval[api]` or `pip install -e ."[api]"`'
            )
        self.requester_args = requester_args
        self.mongo_client = mongo_client
        self.mongo_operator = MongoOperator(client=mongo_client)
        self.wf_id = wf_id
        self.model = model
        self.base_url = base_url
        if not isinstance(batch_size, int) and "auto" in batch_size:
            eval_logger.warning(
                "Automatic batch size is not supported for API models. Defaulting to batch size 1."
            )
        elif int(batch_size) > 1:
            eval_logger.warning(
                "Batch size > 1 detected. Ensure your API supports batched requests with varying total sequence lengths."
            )
        self._batch_size = int(batch_size) if batch_size != "auto" else 1
        self._truncate = truncate
        self._max_gen_toks = int(max_gen_toks)
        self._seed = int(seed)
        # max_length - 1 as we always have 1 token for generation
        eval_logger.info(f"Using max length {max_length} - 1")
        # NOTE (Nicolas): In the TemplateAPI, the it's defined as `self.max_length`, but here we use `_max_length`
        # in order to conciliate with the @property `def max_length` defined in the HFLM class.
        self._max_length = max_length
        if int(num_concurrent) <= 1:
            eval_logger.info(
                "Concurrent requests are disabled. To enable concurrent requests, set `num_concurrent` > 1."
            )
        self._concurrent = int(num_concurrent)
        self.tokenizer_backend = (
            None if tokenizer_backend in ("None", "none") else tokenizer_backend
        )
        self.add_bos_token = add_bos_token
        self.custom_prefix_token_id = custom_prefix_token_id
        self.tokenized_requests = tokenized_requests
        self.max_retries = int(max_retries)
        self.verify_certificate = verify_certificate
        self._eos_string = eos_string
        self.timeout = int(timeout)
        self.max_images = int(max_images)

        # NOTE (nicolas): Due to super.init() is currently not called, we need to set these attributes manually.
        self._rank = 0
        self._world_size = 1

        eval_logger.info(f"Using tokenizer {self.tokenizer_backend}")
        if self.tokenizer_backend is None:
            self.tokenizer = None
            self.tokenized_requests = False
        else:
            eval_logger.info(
                "LM loaded, tokenizer not loaded yet. Will load it from the database."
            )
            ##########################################################
            # Section from TemplateAPI(lm-eval-harness 0.4.9) removed
            #########################################################
            # NOTE (nicolas):
            # When the probabilities are back, then the code related to the tokenizer initialization should go to `load_tokenizer`.

    @cached_property
    def eot_token_id(self) -> Optional[int]:
        if self.tokenizer is None:
            return None
        else:
            if self.tokenizer_backend == "huggingface":
                return self.tokenizer.eos_token_id
            else:
                # raise error
                eval_logger.error(
                    "tokenizer_backend do not supported  (in def eot_token_id)",
                    tokenizer_backend=self.tokenizer_backend,
                )
                raise ApplicationError(
                    "tokenizer_backend do not supported  (in def eot_token_id)",
                    tokenizer_backend=self.tokenizer_backend,
                    non_retryable=True,
                )

    @cached_property
    def eos_string(self) -> Optional[str]:
        if self._eos_string:
            return self._eos_string
        elif self.tokenizer is not None:
            if self.tokenizer_backend == "huggingface":
                return self.tokenizer.eos_token
            # elif self.tokenizer_backend == "tiktoken":
            #    return self.tokenizer.decode([self.tokenizer.eot_token])
            else:
                # raise error
                eval_logger.error(
                    "tokenizer_backend do not supported (in def eos_string)",
                    tokenizer_backend=self.tokenizer_backend,
                )
                raise ApplicationError(
                    "tokenizer_backend do not supported (in def eos_string)",
                    tokenizer_backend=self.tokenizer_backend,
                    non_retryable=True,
                )
        else:
            eval_logger.warning(
                "Cannot determine EOS string to pass to stop sequence. Manually set by passing `eos_string` to model_args."
            )
            return None

    @property
    def max_gen_toks(self) -> int:
        """Maximum number of tokens to generate."""
        return self._max_gen_toks

    @cached_property
    def prefix_token_id(self) -> Optional[int]:
        if self.tokenizer is None:
            return None
        else:
            if self.custom_prefix_token_id is not None:
                return self.custom_prefix_token_id
            if self.tokenizer_backend == "huggingface":
                if self.tokenizer.bos_token_id is not None:
                    return self.tokenizer.bos_token_id
                return self.tokenizer.eos_token_id
            else:
                # raise error
                eval_logger.error(
                    "tokenizer_backend do not supported (in def prefix_token_id)",
                    tokenizer_backend=self.tokenizer_backend,
                )
                raise ApplicationError(
                    "tokenizer_backend do not supported (in def prefix_token_id)",
                    tokenizer_backend=self.tokenizer_backend,
                    non_retryable=True,
                )

    @property
    def max_length(self):
        # if max length manually set or defaulted if no config is available,
        # return it
        if self._max_length:
            return self._max_length
        # next to do apply to POKT benchmark
        # if self.data_parallel_size <= 1:
        #     return self.model.llm_engine.model_config.max_model_len
        else:
            # For instance the file.json generated by AutoConfig is not tracked
            seqlen_config_attrs = ("n_positions", "max_position_embeddings", "n_ctx")
            for attr in seqlen_config_attrs:
                if hasattr(self._config, attr):
                    return getattr(self._config, attr)
            if hasattr(self.tokenizer, "model_max_length"):
                if self.tokenizer.model_max_length == 1000000000000000019884624838656:
                    return self._DEFAULT_MAX_LENGTH
                return self.tokenizer.model_max_length
            return self._DEFAULT_MAX_LENGTH

    @property
    def tokenizer_name(self) -> str:
        return self.tokenizer.name_or_path.replace("/", "__")

    @property
    def batch_size(self) -> int:
        return self._batch_size

    async def load_tokenizer(self) -> bool:
        # -------------------- Load Tokenizer ----------------------------------
        eval_logger.info(f"Using tokenizer {self.tokenizer_backend}")
        if self.tokenizer_backend is None:
            has_tokenizer = False
            self.tokenizer = None
            self.tokenized_requests = False
        else:
            try:
                (
                    has_tokenizer,
                    tokenizer_objects,
                ) = await self.mongo_operator.get_tokenizer_objects(
                    address=self.requester_args.address,
                    service=self.requester_args.service,
                )

            except Exception as e:
                eval_logger.error(
                    "Error loading tokenizer objects",
                    error=str(e),
                    address=self.requester_args.address,
                    service=self.requester_args.service,
                )
                raise ApplicationError(
                    "Error loading tokenizer objects",
                    error=str(e),
                    address=self.requester_args.address,
                    service=self.requester_args.service,
                    non_retryable=True,
                )

            if has_tokenizer:
                try:
                    self.tokenizer = load_tokenizer(
                        tokenizer_objects=tokenizer_objects,
                        wf_id=self.wf_id,
                        trust_remote_code=self.trust_remote_code,
                    )
                except Exception as e:
                    eval_logger.error(
                        "Error loading tokenizer from database",
                        error=str(e),
                        address=self.requester_args.address,
                        service=self.requester_args.service,
                    )
                    raise ApplicationError(
                        "Error loading tokenizer from database",
                        error=str(e),
                        address=self.requester_args.address,
                        service=self.requester_args.service,
                        non_retryable=True,
                    )
            else:
                self.tokenizer = None

        # -------------------- Load Configuration ------------------------------
        try:
            has_config, config_objects = await self.mongo_operator.get_config_objects(
                address=self.requester_args.address,
                service=self.requester_args.service,
            )
        except Exception as e:
            eval_logger.error(
                "Error loading config objects",
                error=str(e),
                address=self.requester_args.address,
                service=self.requester_args.service,
            )
            raise ApplicationError(
                "Error loading config objects",
                error=str(e),
                address=self.requester_args.address,
                service=self.requester_args.service,
                non_retryable=True,
            )

        if has_config:
            try:
                self._config = load_config(
                    config_objects=config_objects,
                    wf_id=self.wf_id,
                    trust_remote_code=False,  # We don't want to download and execute anything
                )
            except Exception as e:
                eval_logger.error(
                    "Error loading config from database",
                    error=str(e),
                    address=self.requester_args.address,
                    service=self.requester_args.service,
                )
                raise ApplicationError(
                    "Error loading config from database",
                    error=str(e),
                    address=self.requester_args.address,
                    service=self.requester_args.service,
                    non_retryable=True,
                )
        else:
            self._config = {}
            self._max_length = self._DEFAULT_MAX_LENGTH

        # -------------------- Update Tokenizer with Config --------------------
        if has_tokenizer and has_config:
            self.tokenizer = configure_pad_token(self.tokenizer)
            self.custom_prefix_token_id = self.init_prefix_token_id
            if self.init_prefix_token_id is not None:
                eval_logger.info(
                    f"Log-likelihood prefix token id used in evaluation: {self.prefix_token_id}"
                )
            self.vocab_size = self.tokenizer.vocab
            self.end_of_text_token_id = self.tokenizer.eos_token_id
            eval_logger.debug(
                "Tokenizer and Config loaded successfully.",
                adress=self.requester_args.address,
                service=self.requester_args.service,
            )
        else:
            self.custom_prefix_token_id = None
            self.init_prefix_token_id = None
            self.vocab_size = None
            self.end_of_text_token_id = None

        # True if tokenizer and config are present, false otherwise
        return has_tokenizer and has_config

    # NOTE (nicolas) `tok_encode` method is now implemented into TemplateAPI`
    # NOTE (nicolas) tok_decode method is now implementd into TemplateAPI, by the name
    # decode_batch

    def model_call(
        self,
        messages: Union[List[List[int]], List[str], List[JsonChatStr]],
        *,
        generate: bool = True,
        gen_kwargs: Optional[Dict] = None,
        **kwargs,
    ) -> Optional[Union[CompletionRequest, ChatCompletionRequest]]:
        # !!! Copy: shared dict for each request, need new object !!!
        gen_kwargs = copy.deepcopy(gen_kwargs)
        try:
            # NOTE: Removed the part related to the request.
            # This class do not performs it, just creates instantes of
            # CompletionRequest or ChatCompletionRequest.
            # response = requests.post(
            # ...
            # return response.json()
            request = self._create_payload_custom(
                messages=messages,
                generate=generate,
                gen_kwargs=gen_kwargs,
                seed=self._seed,
                eos=self.eos_string,
                **kwargs,
            )

            return request
        except RetryError:
            eval_logger.error(
                "API request failed after multiple retries. Please check the API status."
            )
            return None

    def _loglikelihood_tokens(
        self, requests, **kwargs
    ) -> List[Union[ChatCompletionRequest, CompletionRequest]]:
        assert (
            self.tokenizer is not None
        ), "Tokenizer is required for loglikelihood tasks to compute context lengths."
        res = []

        def _collate(req: LogLikelihoodInputs):
            """Defines the key for the sorted method"""
            # the negative sign on len(toks) sorts descending - this has a few advantages:
            # - time estimates will always be over not underestimates, which is more useful for planning
            # - to know the size of a batch when going through the list, you know the first one is always the batch
            #   padded context length. this is useful to simplify the batching logic and more importantly to make
            #   automatic adaptive batches much much easier to implement
            # - any OOMs will happen right away rather than near the end

            toks = req[1] + req[2]
            return -len(toks), tuple(toks)

        re_ord = Collator(
            requests,
            sort_fn=_collate,
            group_by=None,
        )
        # if concurrent then we'll batch in the async context
        chunked = re_ord.get_batched(n=self._batch_size if self._concurrent <= 1 else 0)
        if self._concurrent <= 1:
            pbar = tqdm(desc="Creating requests", total=len(requests))
            for chunk in chunked:
                inputs, ctxlens, _ = self.batch_loglikelihood_requests([chunk])
                eval_logger.error(
                    "Loglikelihood inputs",
                    inputs=inputs,
                    ctxlens=ctxlens,
                )
                outputs = retry(
                    stop=stop_after_attempt(self.max_retries),
                    wait=wait_exponential(multiplier=0.5, min=1, max=10),
                    reraise=True,
                )(self.model_call)(messages=inputs, generate=False)
                if isinstance(outputs, dict):
                    outputs = [outputs]
                # parase log_probs deleted here, do not apply
                for answer_, _ in zip(outputs, inputs, ctxlens):
                    if answer_ is not None:
                        res.append(answer_)
                        pbar.update(1)
        else:
            # For now, raise application error:
            eval_logger.error(
                "Currently, only synchronous requests are supported, please set `num_concurrent=1`."
            )
            return ApplicationError(
                "Currently, only synchronous requests are supported, please set `num_concurrent=1`.",
                non_retryable=True,
            )

        return re_ord.get_original(res)

        # TODO Old code related to _loglikelihood_tokens
        # The logic below should then be addapted into the newer _loglikelihood_tokens method

        # for chunk in tqdm(
        #     list(lm_eval.models.utils.chunks(re_ord.get_reordered(), self.batch_size)),
        #     disable=disable_tqdm,
        # ):
        #     inps = []
        #     ctxlens = []
        #     for cache_key, context_enc, continuation_enc in chunk:
        #         # max_length+1 because the API takes up to 2049 tokens, including the first context token
        #         inp = (context_enc + continuation_enc)[-(self.max_length + 1) :]
        #         # TODO: the logic is much simpler if we just look at the length of continuation tokens
        #         ctxlen = len(context_enc) - max(
        #             0, len(context_enc) + len(continuation_enc) - (self.max_length + 1)
        #         )

        #         inps.append(inp)
        #         ctxlens.append(ctxlen)
        #     ############################################################
        #     # START: POCKET NETWORK CODE
        #     ############################################################
        #     request = CompletionRequest(
        #         model=self.model,
        #         prompt=inps,
        #         echo=True,
        #         max_tokens=0,
        #         temperature=0.0,
        #         logprobs=5,
        #         seed=self.seed,
        #     )

        #     for prompt_i, ctxlen, (cache_key, context_enc, continuation_enc) in zip(
        #         request.prompt, ctxlens, chunk
        #     ):
        #         req_dict = request.to_dict(remove_fields=["prompt"])
        #         req_dict["prompt"] = prompt_i
        #         req_dict["ctxlen"] = ctxlen
        #         req_dict["context_enc"] = context_enc
        #         req_dict["continuation_enc"] = continuation_enc
        #         req_i = CompletionRequest(**req_dict)
        #         res.append(req_i)
        #     ############################################################
        #     # END: POCKET NETWORK CODE
        #     ############################################################

        # return re_ord.get_original(res)

    def generate_until(
        self, requests: List[Instance], disable_tqdm: bool = False
    ) -> List[Union[ChatCompletionRequest, CompletionRequest]]:
        """
        Mix of OpenAI's generate_until and VLLM generate_until
        """
        res = []

        def _collate_gen(_requests):
            # sort by the length of the non-tokenized contexts
            return -len(_requests[0])

        # Let the API deal with tokenization
        if len(requests[0].args) > 2:
            assert (
                self.tokenizer is None
            ), "tokenizer is not supported for multimodal requests yet!"
            eval_logger.info(
                f"Using max_images {self.max_images}. Set in the model args."
            )
            requests, all_gen_kwargs, auxiliary_args = zip(
                *(req.args for req in requests)
            )
            requests = tuple(
                JsonChatStr(
                    json.dumps(
                        create_image_prompt(
                            y["visual"][: self.max_images], json.loads(x.prompt)
                        )
                    )
                )
                for x, y in zip(requests, auxiliary_args)
            )
        else:
            requests, all_gen_kwargs = zip(*(req.args for req in requests))

        if self.tokenized_requests:
            encodings_list = self.tok_encode(
                requests, add_special_tokens=self.add_bos_token
            )
        else:
            encodings_list = [None] * len(requests)
        requests = [
            (a, b, c) for a, b, c in zip(requests, all_gen_kwargs, encodings_list)
        ]

        re_ord = Collator(
            requests,
            sort_fn=_collate_gen,
            group_by="gen_kwargs",
        )
        chunked = re_ord.get_batched(
            n=self._batch_size if self._concurrent <= 1 else 0, batch_fn=None
        )
        if not self.tokenized_requests:
            eval_logger.debug(
                "Tokenized requests are disabled. Context + generation length is not checked."
            )
        if self._concurrent <= 1:
            eval_logger.debug(
                "In generate_until, num_concurrent <= 1",
                num_concurrent=self._concurrent,
            )
            # pbar = tqdm(desc="Requesting API", total=len(requests))
            for chunk in chunked:
                contexts, all_gen_kwargs, encodings_list = zip(*chunk)
                if self.tokenized_requests:
                    max_gen_toks = all_gen_kwargs[0].get(
                        "max_gen_toks", self._max_gen_toks
                    )
                    max_context_len = self.max_length - max_gen_toks

                    encodings_list = [x[-max_context_len:] for x in encodings_list]

                    if any(
                        len(x) + max_gen_toks > self.max_length for x in encodings_list
                    ):
                        eval_logger.warning(
                            f"Some contexts exceeded (max length: ({self.max_length}) - max_gen_toks: ({max_gen_toks}). They were left truncated."
                        )

                req = encodings_list if self.tokenized_requests else contexts
                outputs = retry(
                    stop=stop_after_attempt(self.max_retries),
                    wait=wait_exponential(multiplier=0.5, min=1, max=10),
                    reraise=True,
                )(self.model_call)(
                    messages=req,
                    generate=True,
                    gen_kwargs=copy.deepcopy(all_gen_kwargs[0]),
                )
                eval_logger.debug("generate_until `outputs`", output_type=type(outputs))
                if self.tokenizer is not None:
                    outputs.ctxlen = len(encodings_list)
                    outputs.context_enc = encodings_list
                else:
                    # This is used later to calculate the timeout for the request,
                    # we use an approximation from string to token
                    outputs.estimate_ctxlen()
                res.append(outputs)
                # for generated_text, context in zip(outputs, contexts):
                #     if generated_text is not None:
                #         eval_logger.debug(
                #             "This is what is beeing accumulated in `res`",
                #             generated_text=generated_text,
                #         )
                #         res.append(generated_text)

                #         # partial caching
                #         if context is not None:
                #             # self.cache_hook.add_partial(
                #             #     "generate_until",
                #             #     (context, all_gen_kwargs[0]),
                #             #     generated_text,
                #             # )
                #             pbar.update(1)
        else:
            # For now, raise application error:
            eval_logger.error(
                "Currently, only synchronous requests are supported, please set `num_concurrent=1`."
            )
            return ApplicationError(
                "Currently, only synchronous requests are supported, please set `num_concurrent=1`.",
                non_retryable=True,
            )

        return re_ord.get_original(res)

    def loglikelihood(
        self, requests, disable_tqdm: bool = True
    ) -> List[CompletionRequest]:
        # TODO: Currently loglikelihood is not supporteed
        # In the future re-adapt the logic to the new API
        # new_reqs = []
        # for context, continuation in [req.args for req in requests]:
        #     if context == "":
        #         # BOS or EOS as context
        #         context_enc, continuation_enc = (
        #             [self.prefix_token_id],
        #             self.tok_encode(continuation),
        #         )
        #     else:
        #         context_enc, continuation_enc = self._encode_pair(context, continuation)

        #     new_reqs.append(((context, continuation), context_enc, continuation_enc))

        # return self._loglikelihood_tokens(new_reqs, disable_tqdm=disable_tqdm)

        # Raise error if loglikelihood is not supported
        self.eval_logger.error("Loglikelihood `output_type` currently is not supported")
        raise ApplicationError(
            "Loglikelihood `output_type` currently is not supported",
            non_retryable=True,
        )


class SamplerCompletionAPI(SamplerAPI, LocalCompletionsAPI):
    def __init__(
        self,
        requester_args: RequesterArgs,
        mongo_client: MongoClient,
        wf_id: str,
        **kwargs,
    ):
        super().__init__(
            requester_args=requester_args,
            mongo_client=mongo_client,
            wf_id=wf_id,
            **kwargs,
        )

    def _create_payload_custom(
        self,
        messages: Union[List[List[int]], List[dict], List[str], str],
        generate=False,
        gen_kwargs: Optional[dict] = None,
        seed: int = 1234,
        eos=None,
        **kwargs,
    ) -> CompletionRequest:
        request = self._create_payload(
            self.create_message(messages),
            generate=generate,
            gen_kwargs=gen_kwargs,
            seed=self._seed,
            eos=self.eos_string,
            **kwargs,
        )
        # Return CompletionRequest instance
        return CompletionRequest(**request)


class SamplerChatCompletionAPI(SamplerAPI, LocalChatCompletion):
    def __init__(
        self,
        requester_args: RequesterArgs,
        mongo_client: MongoClient,
        wf_id: str,
        **kwargs,
    ):
        super().__init__(
            requester_args=requester_args,
            mongo_client=mongo_client,
            wf_id=wf_id,
            **kwargs,
        )

    def _create_payload_custom(
        self,
        messages: List[Dict],
        generate=False,
        gen_kwargs: dict = None,
        seed=1234,
        eos=None,
        **kwargs,
    ) -> ChatCompletionRequest:
        request = self._create_payload(
            self.create_message(messages),
            generate=generate,
            gen_kwargs=gen_kwargs,
            seed=self._seed,
            eos=self.eos_string,
            **kwargs,
        )
        # Return CompletionRequest instance
        return ChatCompletionRequest(**request)


class EvaluatorAPI(TemplateAPI):
    _DEFAULT_MAX_LENGTH = GENERATION_MAX_LENGHT

    def __init__(
        self,
        truncate: bool = False,
        max_gen_toks: int = GENERATION_MAX_LENGHT,
        batch_size: int = 1,
        seed: int = 1234,
        max_length: Optional[int] = None,
        num_concurrent: int = 1,
        max_retries: int = 3,
        tokenized_requests: bool = False,
        tokenizer_backend: Optional[
            Literal["huggingface", "None", "none"]
        ] = "huggingface",
        trust_remote_code: bool = False,
    ) -> None:
        """
        :param truncate: bool
            Truncate input if too long (if False and input is too long, throw error)
        """
        # super().__init__()
        self.seed = seed
        self.truncate = truncate
        self._batch_size = batch_size
        self._max_gen_toks = max_gen_toks
        self._max_length = max_length
        # Others params needeed for the API
        self._concurrent = int(num_concurrent)
        self.max_retries = int(max_retries)
        self.tokenized_requests = tokenized_requests
        self.tokenizer_backend = (
            None if tokenizer_backend in ("None", "none") else tokenizer_backend
        )
        self.trust_remote_code = trust_remote_code

    def model_call(
        self,
        messages: Union[
            List[CompletionResponse],
            List[ChatCompletionResponse],
            CompletionResponse,
            ChatCompletionResponse,
        ],
        *,
        generate: bool = True,
        gen_kwargs: Optional[Dict] = None,
        **kwargs,
    ) -> Optional[dict]:
        # !!! Copy: shared dict for each request, need new object !!!
        gen_kwargs = copy.deepcopy(gen_kwargs)
        try:
            if isinstance(messages, (CompletionResponse, ChatCompletionResponse)):
                response = messages.model_dump()
            elif isinstance(messages, list):
                response = [msg.model_dump() for msg in messages]
            else:
                evaluation_logger.error(
                    "Invalid type for messages. Expected CompletionResponse or ChatCompletionResponse or list of them.",
                    messages_type=type(messages),
                    messages=messages,
                )
                raise ApplicationError(
                    "Invalid type for messages. Expected CompletionResponse or ChatCompletionResponse or list of them.",
                    non_retryable=True,
                )
            return response

        except RetryError:
            evaluation_logger.error(
                "API request failed after multiple retries. Please check the API status."
            )
            return None

    def generate_until(
        self, requests: List[Instance], disable_tqdm: bool = False
    ) -> List[str]:
        res = []

        def _collate_gen(_requests):
            # sort by the length of the non-tokenized contexts
            return -len(_requests[0])

        # Let the API deal with tokenization
        if len(requests[0].args) > 2:
            pass
            # This should be handleed previously in the sampler.
        else:
            # NOTE: This line was modified w.r.t to the Sampler due to
            # when saved into mongo, the arguments add one extra dim into the list.
            # So, we remove them, and also incorporate the responses
            # Extract data from requests properly
            requests, all_gen_kwargs, resps = zip(
                *((*req.args, req.resp) for req in requests)
            )
            evaluation_logger.debug(
                "generate_until `requests`",
                requests=requests,
                all_gen_kwargs=all_gen_kwargs,
                resps=resps,
            )

        # NOTE: in generate_until we do not use tokenization, so
        # encodings_list is not used a list of Nones.
        if self.tokenized_requests:
            encodings_list = self.tok_encode(
                requests, add_special_tokens=self.add_bos_token
            )
        else:
            encodings_list = [None] * len(requests)

        requests = [
            (a, b, c, d)
            for a, b, c, d in zip(requests, all_gen_kwargs, encodings_list, resps)
        ]

        re_ord = Collator(
            requests,
            sort_fn=_collate_gen,
            group_by="gen_kwargs",
        )
        chunked = re_ord.get_batched(
            n=self._batch_size if self._concurrent <= 1 else 0, batch_fn=None
        )
        if not self.tokenized_requests:
            evaluation_logger.debug(
                "Tokenized requests are disabled. Context + generation length is not checked."
            )
        if self._concurrent <= 1:
            for chunk in chunked:
                contexts, all_gen_kwargs, encodings_list, resp = zip(*chunk)
                ####################################
                # Section w.r.t to tokenizer removed
                ####################################
                # NOTE: At this point, the req variable should be directly the .resp attr of the isntance.
                # req = encodings_list if self.tokenized_requests else contexts
                req = list(resp)
                outputs = retry(
                    stop=stop_after_attempt(self.max_retries),
                    wait=wait_exponential(multiplier=0.5, min=1, max=3),
                    reraise=True,
                )(self.model_call)(
                    messages=req,
                    generate=True,
                    gen_kwargs=copy.deepcopy(all_gen_kwargs[0]),
                )
                for generated_text, _ in zip(
                    self.parse_generations(
                        outputs=outputs,
                        contexts=contexts,
                    ),
                    contexts,
                ):
                    if generated_text is not None:
                        res.append(generated_text)
                    ####################################
                    # section w.r.t to caching removed
                    ####################################
        else:
            # For now, raise application error:
            evaluation_logger.error(
                "Currently, only synchronous requests are supported, please set `num_concurrent=1`."
            )
            return ApplicationError(
                "Currently, only synchronous requests are supported, please set `num_concurrent=1`.",
                non_retryable=True,
            )

        return re_ord.get_original(res)

    def loglikelihood_rolling(self, requests, disable_tqdm: bool = True) -> List[float]:
        # TODO: Update this method in order to be available for the Pocket Network
        return ApplicationError(
            "Currently evaluation of task with loglikelihood_rolling are not suported",
            non_retryable=True,
        )

    def loglikelihood(
        self, requests, disable_tqdm: bool = True
    ) -> List[CompletionRequest]:
        # Modify this in order to insted of get contex and continuation,
        # get the context, continuation, context_enc and continuation_enc.
        # then new_reqs should have the responses.

        new_reqs = []
        for ([context, continuation],), context_enc, continuation_enc, resp in [
            (req.args, req.prompt.context_enc, req.prompt.continuation_enc, req.resp)
            for req in requests
        ]:
            new_reqs.append(
                ((context, continuation), context_enc, continuation_enc, resp)
            )

        return self._loglikelihood_tokens(new_reqs, disable_tqdm=disable_tqdm)

    def response_times(self, requests, disable_tqdm: bool = True) -> List[int]:
        return [req.resp.response_time for req in requests]

    def _encode_pair(self, context_enc, continuation_enc):
        return context_enc, continuation_enc


class EvaluatorCompletion(EvaluatorAPI, LocalCompletionsAPI):
    """
    Evaluator for completion models, that define the 'self.parse_generations' method.
    """

    MULTIMODAL = False

    def __init__(
        self,
        truncate: bool = False,
        max_gen_toks: int = GENERATION_MAX_LENGHT,
        batch_size: int = 1,
        seed: int = 1234,
        max_length: Optional[int] = None,
        num_concurrent: int = 1,
        max_retries: int = 3,
        tokenized_requests: bool = False,
        tokenizer_backend: Optional[
            Literal["huggingface", "None", "none"]
        ] = "huggingface",
        trust_remote_code: bool = False,
    ) -> None:
        # Only initialize EvaluatorAPI to avoid conflicts.
        # The herency of LocalCompletionsAPI is only to use the `parse_generations` and `parse_logprobs`
        #  methods.
        EvaluatorAPI.__init__(
            self,
            truncate=truncate,
            max_gen_toks=max_gen_toks,
            batch_size=batch_size,
            seed=seed,
            max_length=max_length,
            num_concurrent=num_concurrent,
            max_retries=max_retries,
            tokenized_requests=tokenized_requests,
            tokenizer_backend=tokenizer_backend,
            trust_remote_code=trust_remote_code,
        )

        self._rank = 0
        self._world_size = 1

    @staticmethod
    def parse_generations(outputs: Union[Dict, List[Dict]], **kwargs) -> List[str]:
        res = []
        if not isinstance(outputs, list):
            outputs = [outputs]
        for out in outputs:
            tmp = [None] * len(out["choices"])
            for choices in out["choices"]:
                #########################################
                # START: CUSTOM CODE
                #########################################
                # NOTE (nicolas) See comments in EvaluatorChatCompletion.parse_generations
                content = choices["text"]
                if content is None or content == "":
                    content = INVALID_ANSWER
                tmp[choices["index"]] = content
                ######################################
                # END: CUSTOM CODE
                ######################################
            res = res + tmp
        return res


class EvaluatorChatCompletion(EvaluatorAPI, LocalChatCompletion):
    MULTIMODAL = False

    """
    Evaluator for chat models, that define the 'self.parse_generations' method.
    """

    def __init__(
        self,
        truncate: bool = False,
        max_gen_toks: int = GENERATION_MAX_LENGHT,
        batch_size: int = 1,
        seed: int = 1234,
        max_length: Optional[int] = None,
        num_concurrent: int = 1,
        max_retries: int = 3,
        tokenized_requests: bool = False,
        tokenizer_backend: Optional[
            Literal["huggingface", "None", "none"]
        ] = "huggingface",
        trust_remote_code: bool = False,
    ) -> None:
        # Only initialize EvaluatorAPI to avoid conflicts.
        # The herency of LocalChatCompletion is only to use the parse_generations method.
        EvaluatorAPI.__init__(
            self,
            truncate=truncate,
            max_gen_toks=max_gen_toks,
            batch_size=batch_size,
            seed=seed,
            max_length=max_length,
            num_concurrent=num_concurrent,
            max_retries=max_retries,
            tokenized_requests=tokenized_requests,
            tokenizer_backend=tokenizer_backend,
            trust_remote_code=trust_remote_code,
        )
        self._rank = 0
        self._world_size = 1

    # from LocalChatCompletion.parse_generations
    @staticmethod
    def parse_generations(outputs: Union[Dict, List[Dict]], **kwargs) -> List[str]:
        evaluation_logger.debug(
            "Parsing generations from outputs",
            outputs=outputs,
            kwargs=kwargs,
        )
        res = []
        if not isinstance(outputs, list):
            outputs = [outputs]
        for out in outputs:
            tmp = [None] * len(out["choices"])
            for choices in out["choices"]:
                #########################################
                # START: CUSTOM CODE
                #########################################
                # NOTE (nicolas) This sections handle
                # cases where content could be in
                # * `reasoning content`, or in response
                # where the first generation code whas a stop secuences of character and then
                # the content is empty (probably abusing the stop sequence).
                # `stop_reason field
                content = choices["message"]["content"]
                if content is None or content == "":
                    content = INVALID_ANSWER
                tmp[choices["index"]] = content
                ######################################
                # END: CUSTOM CODE
                ######################################
            res = res + tmp
        return res
