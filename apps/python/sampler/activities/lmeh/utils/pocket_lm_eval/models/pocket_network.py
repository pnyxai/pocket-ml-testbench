# Adapted from lm_eval/models/openai_completions.py
import os
import openai  # noqa: E401
from typing import List, Optional

from tqdm import tqdm

import lm_eval.models.utils
from lm_eval import utils
from lm_eval.api.model import TemplateLM

import transformers
from activities.lmeh.utils.mongodb import get_tokenizer_objects
from activities.lmeh.utils.tokenizers import load_tokenizer
from app.app import get_app_logger
from protocol.protocol import RequesterArgs, CompletionRequest
from pymongo import MongoClient
from temporalio.exceptions import ApplicationError
eval_logger = get_app_logger("sample")


class PocketNetworkLM(TemplateLM):
    _DEFAULT_MAX_LENGTH = 2048

    def __init__(
        self,
        requester_args: RequesterArgs,
        mongo_client: MongoClient,
        model: str="pocket_network",
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

        # Load tokenizer
        try:
            tokenizer_objects = get_tokenizer_objects(
                adress=requester_args.address, 
                service=requester_args.service, 
                client=mongo_client,
                )
            self.tokenizer = load_tokenizer(tokenizer_objects)
            self.vocab_size = self.tokenizer.vocab
            self.end_of_text_token_id = self.tokenizer.eos_token
            eval_logger.info(f"Tokenizer loaded successfully.")
        except Exception as e:
            raise e
        
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
        self, requests, disable_tqdm: bool = False
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
                req_i = CompletionRequest(**req_dict)
                res.append(req_i)
            ############################################################
            # END: POCKET NETWORK CODE
            ############################################################

        return re_ord.get_original(res)

    def generate_until(self, requests, disable_tqdm: bool = False) -> List[CompletionRequest]:
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

            until = request_args.get("until", ["<|endoftext|>"])
            request_args["temperature"] = request_args.get("temperature", 0)
            ############################################################
            # START: POCKET NETWORK CODE
            ############################################################
            extra_args = {
                    k: v
                    for k, v in request_args.items()
                    if k not in ["do_sample", "max_gen_toks", "until"]
                }
            eval_logger.debug("CompletionRequest: ", model=self.model, prompt=inps, max_tokens=self.max_gen_toks, stop=until, seed=self.seed)
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
                req_dict["prompt"] = prompt_i
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

    def loglikelihood_rolling(
        self, requests, disable_tqdm: bool = False
    ) -> List[float]:
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
        self, requests, disable_tqdm: bool = False
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