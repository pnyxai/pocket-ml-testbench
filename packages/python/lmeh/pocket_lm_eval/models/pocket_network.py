# Adapted from lm_eval/models/openai_completions.py
import copy
import transformers

from typing import List, Optional, Tuple
from tqdm import tqdm
import lm_eval.models.utils
from lm_eval import utils
from lm_eval.api.model import TemplateLM
from lm_eval.models.openai_completions import get_result
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.lmeh.utils.tokenizers import load_tokenizer
from app.app import get_app_logger
from packages.python.protocol.protocol import RequesterArgs, CompletionRequest
from packages.python.common.mongodb import MongoClient
from temporalio.exceptions import ApplicationError

eval_logger = get_app_logger("sample")
evaluation_logger = get_app_logger("evaluation")


class PocketNetworkLM(TemplateLM):
    _DEFAULT_MAX_LENGTH = 4096

    def __init__(
        self,
        requester_args: RequesterArgs,
        mongo_client: MongoClient,
        wf_id: str,
        model: str = "pocket_network",
        base_url: str = None,
        truncate: bool = False,
        max_gen_toks: int = 256,
        batch_size: int = 1,
        seed: int = 1234,
        max_length: Optional[int] = None,
    ) -> None:
        """

        :param truncate: bool
            Truncate input if too long (if False and input is too long, throw error)
        """
        super().__init__()
        self.seed = seed
        self.model = model
        self.base_url = base_url
        self.truncate = truncate
        self._batch_size = batch_size
        self._max_gen_toks = max_gen_toks
        self._max_length = max_length
        self.wf_id = wf_id
        self.requester_args = requester_args
        self.mongo_client = mongo_client
        self.mongo_operator = MongoOperator(client=mongo_client)

    @property
    def eot_token_id(self):
        return self.end_of_text_token_id

    @property
    def max_length(self) -> int:
        if self._max_length:
            return self._max_length
        else:
            return self._DEFAULT_MAX_LENGTH

    @property
    def max_gen_toks(self) -> int:
        return self._max_gen_toks

    @property
    def batch_size(self) -> int:
        return self._batch_size

    @property
    def device(self):
        # Isn't used because we override _loglikelihood_tokens
        raise NotImplementedError()

    async def load_tokenizer(self):
        # Load tokenizer
        tokenizer_objects = await self.mongo_operator.get_tokenizer_objects(
            address=self.requester_args.address,
            service=self.requester_args.service,
        )
        self.tokenizer = load_tokenizer(
            tokenizer_objects=tokenizer_objects,
            wf_id=self.wf_id,
        )
        self.vocab_size = self.tokenizer.vocab
        self.end_of_text_token_id = self.tokenizer.eos_token_id
        eval_logger.debug(
            "Tokenizer loaded successfully.",
            adress=self.requester_args.address,
            service=self.requester_args.service,
        )

    def tok_encode(self, string: str, **kwargs) -> List[int]:
        # TODO: Add options like in lm_eval/models/vllm_causallms.py
        if not self.tokenizer:
            raise "must call await <instance>.load_tokenizer()"
        return self.tokenizer.encode(string)

    def tok_decode(self, tokens: List[int]) -> str:
        if not self.tokenizer:
            raise "must call await <instance>.load_tokenizer()"
        return self.tokenizer.decode(tokens)

    def _loglikelihood_tokens(
        self, requests, disable_tqdm: bool = True
    ) -> List[CompletionRequest]:
        res = []

        def _collate(x):
            # this doesn't efficiently handle last-token differences yet, but those are kinda annoying because
            # it's not guaranteed that the 100 or so logprobs we get to see actually contain all the continuations
            # we care about, and so we need some kind of backup for when it isn't
            toks = x[1] + x[2]
            return -len(toks), tuple(toks)

        re_ord = utils.Reorderer(requests, _collate)

        for chunk in tqdm(
            list(lm_eval.models.utils.chunks(re_ord.get_reordered(), self.batch_size)),
            disable=disable_tqdm,
        ):
            inps = []
            ctxlens = []
            for cache_key, context_enc, continuation_enc in chunk:
                # max_length+1 because the API takes up to 2049 tokens, including the first context token
                inp = (context_enc + continuation_enc)[-(self.max_length + 1) :]
                # TODO: the logic is much simpler if we just look at the length of continuation tokens
                ctxlen = len(context_enc) - max(
                    0, len(context_enc) + len(continuation_enc) - (self.max_length + 1)
                )

                inps.append(inp)
                ctxlens.append(ctxlen)
            ############################################################
            # START: POCKET NETWORK CODE
            ############################################################
            request = CompletionRequest(
                model=self.model,
                prompt=inps,
                echo=True,
                max_tokens=0,
                temperature=0.0,
                logprobs=5,
                seed=self.seed,
            )

            for prompt_i, ctxlen, (cache_key, context_enc, continuation_enc) in zip(
                request.prompt, ctxlens, chunk
            ):
                req_dict = request.to_dict(remove_fields=["prompt"])
                req_dict["prompt"] = prompt_i
                req_dict["ctxlen"] = ctxlen
                req_dict["context_enc"] = context_enc
                req_dict["continuation_enc"] = continuation_enc
                req_i = CompletionRequest(**req_dict)
                res.append(req_i)
            ############################################################
            # END: POCKET NETWORK CODE
            ############################################################

        return re_ord.get_original(res)

    def generate_until(
        self, requests, disable_tqdm: bool = True
    ) -> List[CompletionRequest]:
        if not requests:
            return []
        res = []
        requests = [req.args for req in requests]

        def _collate(x):
            toks = self.tok_encode(x[0])
            return len(toks), x[0]

        re_ord = utils.Reorderer(requests, _collate)

        def sameuntil_chunks(xs, size):
            ret = []
            lastuntil = xs[0][1]
            for x in xs:
                if len(ret) >= size or x[1] != lastuntil:
                    yield ret, lastuntil
                    ret = []
                    lastuntil = x[1]
                ret.append(x)

            if ret:
                yield ret, lastuntil

        # todo: more intelligent batching for heterogeneous `until`
        for chunk, request_args in tqdm(
            list(sameuntil_chunks(re_ord.get_reordered(), self.batch_size)),
            disable=disable_tqdm,
        ):
            inps = []
            self._max_gen_toks = request_args.get("max_gen_toks", self.max_gen_toks)
            for context, _ in chunk:
                context_enc = self.tok_encode(context)
                inp = context_enc[-(self.max_length - self.max_gen_toks) :]
                inps.append(inp)
            gen_kwargs = request_args
            until = None
            if isinstance(gen_kwargs, dict):
                kwargs = copy.deepcopy(gen_kwargs)  # edge case for repeats > 1
                if "until" in kwargs.keys():
                    until = kwargs.pop("until")
                    if isinstance(until, str):
                        until = [until]
                    elif not isinstance(until, list):
                        raise ValueError(
                            f"Expected `kwargs['until']` to be of type Union[str,list] but got {until}"
                        )
            else:
                raise ValueError(
                    f"Expected `kwargs` to be of type `dict` but got {gen_kwargs}"
                )
            # add EOS token to stop sequences
            eos = self.tokenizer.decode(self.eot_token_id)
            if not until:
                until = [eos]
            else:
                until.append(eos)
            request_args["temperature"] = request_args.get("temperature", 0)
            ############################################################
            # START: POCKET NETWORK CODE
            ############################################################
            extra_args = {
                k: v
                for k, v in request_args.items()
                if k not in ["do_sample", "max_gen_toks", "until"]
            }
            eval_logger.debug(
                "CompletionRequest: ",
                model=self.model,
                prompt=inps,
                max_tokens=self.max_gen_toks,
                stop=until,
                seed=self.seed,
            )
            eval_logger.debug("Extra args: ", **extra_args)
            request = CompletionRequest(
                model=self.model,
                prompt=inps,
                max_tokens=self.max_gen_toks,
                stop=until,
                seed=self.seed,
                **extra_args,
            )
            for prompt_i, (context, args_) in zip(request.prompt, chunk):
                req_dict = request.to_dict(remove_fields=["prompt"])
                # context is a string
                req_dict["prompt"] = prompt_i
                req_dict["ctxlen"] = len(prompt_i)
                req_dict["context_enc"] = prompt_i
                req_i = CompletionRequest(**req_dict)
                res.append(req_i)
            ############################################################
            # END: POCKET NETWORK CODE
            ############################################################
        return re_ord.get_original(res)

    def _model_call(self, inps):
        # Isn't used because we override _loglikelihood_tokens
        raise NotImplementedError()

    def _model_generate(self, context, max_length, eos_token_id):
        # Isn't used because we override generate_until
        raise NotImplementedError()

    def loglikelihood_rolling(self, requests, disable_tqdm: bool = True) -> List[float]:
        loglikelihoods = []

        for (string,) in tqdm([req.args for req in requests], disable=disable_tqdm):
            rolling_token_windows = list(
                map(
                    utils.make_disjoint_window,
                    utils.get_rolling_token_windows(
                        token_list=self.tok_encode(string),
                        prefix_token=self.eot_token_id,
                        max_seq_len=self.max_length,
                        context_len=1,
                    ),
                )
            )

            # TODO: Right now, we pass single EOT token to the Encoder and the full context to the decoder, in seq2seq case
            rolling_token_windows = [(None,) + x for x in rolling_token_windows]

            string_nll = self._loglikelihood_tokens(
                rolling_token_windows,
                disable_tqdm=True,
            )

            # discard is_greedy
            string_nll = [x[0] for x in string_nll]

            string_nll = sum(string_nll)
            loglikelihoods.append(string_nll)
        return loglikelihoods

    def loglikelihood(
        self, requests, disable_tqdm: bool = True
    ) -> List[CompletionRequest]:
        new_reqs = []
        for context, continuation in [req.args for req in requests]:
            if context == "":
                # BOS or EOS as context
                context_enc, continuation_enc = (
                    [self.prefix_token_id],
                    self.tok_encode(continuation),
                )
            else:
                context_enc, continuation_enc = self._encode_pair(context, continuation)

            new_reqs.append(((context, continuation), context_enc, continuation_enc))

        return self._loglikelihood_tokens(new_reqs, disable_tqdm=disable_tqdm)

    # TODO: remove def _encode_pair to follow lm-eval-harness>0.4.2.
    def _encode_pair(self, context, continuation):
        n_spaces = len(context) - len(context.rstrip())
        if n_spaces > 0:
            continuation = context[-n_spaces:] + continuation
            context = context[:-n_spaces]

        model_class = getattr(self, "AUTO_MODEL_CLASS", None)

        if model_class == transformers.AutoModelForSeq2SeqLM:
            context_enc = self.tok_encode(context)
            continuation_enc = self.tok_encode(continuation, add_special_tokens=False)
        else:
            whole_enc = self.tok_encode(context + continuation)
            context_enc = self.tok_encode(context)

            context_enc_len = len(context_enc)
            continuation_enc = whole_enc[context_enc_len:]

        return context_enc, continuation_enc


class EvaluatorLM(TemplateLM):
    _DEFAULT_MAX_LENGTH = 4096

    def __init__(
        self,
        truncate: bool = False,
        max_gen_toks: int = 256,
        batch_size: int = 1,
        seed: int = 1234,
        max_length: Optional[int] = None,
    ) -> None:
        """
        :param truncate: bool
            Truncate input if too long (if False and input is too long, throw error)
        """
        super().__init__()
        self.seed = seed
        self.truncate = truncate
        self._batch_size = batch_size
        self._max_gen_toks = max_gen_toks
        self._max_length = max_length

    @property
    def eot_token_id(self):
        return self.end_of_text_token_id

    @property
    def max_length(self) -> int:
        if self._max_length:
            return self._max_length
        else:
            return self._DEFAULT_MAX_LENGTH

    @property
    def max_gen_toks(self) -> int:
        return self._max_gen_toks

    @property
    def batch_size(self) -> int:
        return self._batch_size

    @property
    def device(self):
        # Isn't used because we override _loglikelihood_tokens
        raise NotImplementedError()

    def tok_encode(self, string: str, **kwargs) -> List[int]:
        return self.tokenizer.encode(string)

    def tok_decode(self, tokens: List[int]) -> str:
        return self.tokenizer.decode(tokens)

    def _loglikelihood_tokens(
        self, requests, disable_tqdm: bool = True
    ) -> List[Tuple[float, bool]]:
        res = []

        def _collate(x):
            # this doesn't efficiently handle last-token differences yet, but those are kinda annoying because
            # it's not guaranteed that the 100 or so logprobs we get to see actually contain all the continuations
            # we care about, and so we need some kind of backup for when it isn't
            toks = x[1] + x[2]
            return -len(toks), tuple(toks)

        re_ord = utils.Reorderer(requests, _collate)

        for chunk in tqdm(
            list(lm_eval.models.utils.chunks(re_ord.get_reordered(), self.batch_size)),
            disable=disable_tqdm,
        ):
            inps = []
            ctxlens = []
            response = []
            for cache_key, context_enc, continuation_enc, resp in chunk:
                # max_length+1 because the API takes up to 2049 tokens, including the first context token
                inp = (context_enc + continuation_enc)[-(self.max_length + 1) :]
                # TODO: the logic is much simpler if we just look at the length of continuation tokens
                ctxlen = len(context_enc) - max(
                    0, len(context_enc) + len(continuation_enc) - (self.max_length + 1)
                )
                inps.append(inp)
                ctxlens.append(ctxlen)
                response.append(resp)

            for resp, ctxlen, (cache_key, context_enc, continuation_enc, resp) in zip(
                response, ctxlens, chunk
            ):
                answer = get_result(resp.choices[0], ctxlen)

                res.append(answer)
        return re_ord.get_original(res)

    def generate_until(
        self, requests, disable_tqdm: bool = True
    ) -> List[CompletionRequest]:
        if not requests:
            return []
        res = []
        # batch tokenize contexts
        context, all_gen_kwargs = zip(*(req.args[0] for req in requests))
        context_encoding = [req.prompt.context_enc for req in requests]
        responses = [req.resp for req in requests]
        completion_requests = [req.prompt.data for req in requests]
        requests = [
            ((a, b, cr, r), c)
            for a, b, cr, r, c in zip(
                context,
                context_encoding,
                completion_requests,
                responses,
                all_gen_kwargs,
            )
        ]
        evaluation_logger.debug("Qty of requests: ", qty_req=len(requests))

        def _collate(x):
            toks = x[0][1]
            return len(toks), x[0][0]

        re_ord = utils.Reorderer(requests, _collate)

        def sameuntil_chunks(xs, size):
            ret = []
            lastuntil = xs[0][1]
            for x in xs:
                if len(ret) >= size or x[1] != lastuntil:
                    yield ret, lastuntil
                    ret = []
                    lastuntil = x[1]
                ret.append(x)

            if ret:
                yield ret, lastuntil

        # todo: more intelligent batching for heterogeneous `until`
        for chunk, request_args in tqdm(
            list(sameuntil_chunks(re_ord.get_reordered(), self.batch_size)),
            disable=disable_tqdm,
        ):
            context, _, completion_request, response = chunk[0][0]

            until = completion_request.stop
            for resp, (context, args_) in zip(response.choices, chunk):
                s = getattr(resp, "text")

                until_ = until

                for term in until_:
                    if len(term) > 0:
                        s = s.split(term)[0]
                res.append(s)  #
        return re_ord.get_original(res)

    def _model_call(self, inps):
        # Isn't used because we override _loglikelihood_tokens
        raise NotImplementedError()

    def _model_generate(self, context, max_length, eos_token_id):
        # Isn't used because we override generate_until
        raise NotImplementedError()

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

    def _encode_pair(self, context_enc, continuation_enc):
        return context_enc, continuation_enc
