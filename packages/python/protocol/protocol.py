import time
import uuid
from datetime import datetime
from typing import Any, Callable, Dict, List, Literal, Optional, Union

from bson import ObjectId
from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator


######################
# REGISTER / REQUESTER
######################
class PocketNetworkRegisterTaskRequest(BaseModel):
    framework: str
    tasks: str
    verbosity: Optional[Literal["CRITICAL", "ERROR", "WARNING", "INFO", "DEBUG"]] = (
        "ERROR"
    )
    include_path: Optional[str] = None


class RequesterArgs(BaseModel):
    address: str
    service: str
    method: str = "POST"
    path: str = "/v1/completions"
    headers: Optional[Dict] = {"Content-Type": "application/json"}


class PocketNetworkTaskRequest(PocketNetworkRegisterTaskRequest):
    requester_args: RequesterArgs
    blacklist: Optional[List[int]] = []
    qty: Optional[int] = None
    doc_ids: Optional[List[int]] = None
    model: Optional[str] = "pocket_network"
    llm_args: Optional[Dict] = (
        None  # TODO : Remove: This is LLM specific, move to agnostic format.
    )
    num_fewshot: Optional[int] = Field(
        None, ge=0
    )  # TODO : Remove: This is LLM specific, move to agnostic format.
    gen_kwargs: Optional[str] = None
    bootstrap_iters: Optional[int] = 100000
    system_instruction: Optional[str] = None
    apply_chat_template: Optional[bool] = False
    fewshot_as_multiturn: Optional[bool] = False

    @model_validator(mode="after")
    def verify_qty_or_doc_ids(self):
        if (self.qty and self.doc_ids) or (not self.qty and not self.doc_ids):
            raise ValueError("Expected qty or doc_ids but not both.")
        return self

    @model_validator(mode="after")
    def remove_blacklist_when_all(self):
        if self.qty < 0:
            self.blacklist = []
            self.doc_ids = None
        return self

    # TODO: Fix this, problem between pydantic and temporalio
    # @model_validator(mode="after")
    # def verify_blacklist_with_doc_ids(self):
    #     if (self.doc_ids and self.blacklist):
    #         if any([x in self.doc_ids for x in self.blacklist]):
    #             raise ValueError("Elements in blacklist must not be in doc_ids")

    # TODO: validate that tasks field is unique in task sense,


class PyObjectId(ObjectId):
    @classmethod
    def __get_validators__(cls):
        yield cls.validate

    @classmethod
    def validate(cls, v):
        if not isinstance(v, ObjectId):
            raise ValueError("Not a valid ObjectId")
        return str(v)


# From vllm/entrypoints/openai/protocol.py
class OpenAIBaseModel(BaseModel):
    # OpenAI API does not allow extra fields
    model_config = ConfigDict(extra="forbid")


class CompletionRequest(BaseModel):
    # Ordered by official OpenAI API documentation
    # https://platform.openai.com/docs/api-reference/completions/create
    model: str
    prompt: Union[List[int], List[List[int]], str, List[str]]
    best_of: Optional[int] = None
    echo: Optional[bool] = False
    frequency_penalty: Optional[float] = 0.0
    logit_bias: Optional[Dict[str, float]] = None
    logprobs: Optional[int] = None
    max_tokens: Optional[int] = 16
    n: int = 1
    presence_penalty: Optional[float] = 0.0
    seed: Optional[int] = Field(
        None,
        ge=-9223372036854775808,  # from torch.iinfo(torch.long).min,
        le=9223372036854775807,
    )  # from torch.iinfo(torch.long).max)
    stop: Optional[Union[str, List[str]]] = Field(default_factory=list)
    stream: Optional[bool] = False
    suffix: Optional[str] = None
    temperature: Optional[float] = 1.0
    top_p: Optional[float] = 1.0
    user: Optional[str] = None
    # Fields to avoid futures tokenizer's call
    ctxlen: Optional[int] = None
    context_enc: Optional[List[int]] = None
    continuation_enc: Optional[List[int]] = None

    def to_dict(self, remove_fields: Optional[List[str]] = None):
        data = self.model_dump(exclude_defaults=True)
        if remove_fields:
            for field in remove_fields:
                if field in data:
                    del data[field]
        return data


# This class serves a subgroup of prompts, as a task can have many instances,
# and each instance has many prompts.
# If not all the prompts are finished, an instance cannot be finished.
# The actual record contains many more optional (and variable) fields, but these
# are mandatory.
class PocketNetworkMongoDBInstance(BaseModel):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    done: bool = False
    # -- Relations Below --
    task_id: ObjectId

    class Config:
        arbitrary_types_allowed = True


class PocketNetworkMongoDBPrompt(BaseModel):
    model_config = ConfigDict(arbitrary_types_allowed=True)
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    data: Union[str, CompletionRequest]
    task_id: ObjectId
    instance_id: ObjectId
    timeout: int = 0
    done: bool = False
    # Fields to avoid futures tokenizer's call
    ctxlen: Optional[int] = None
    context_enc: Optional[List[int]] = None
    continuation_enc: Optional[List[int]] = None


class PocketNetworkMongoDBTask(BaseModel):
    framework: str
    requester_args: RequesterArgs
    blacklist: Optional[List[int]] = []
    llm_args: Optional[Dict] = (
        None  # TODO : Remove: This is LLM specific, move to agnostic format.
    )
    num_fewshot: Optional[int] = Field(
        None, ge=0
    )  # TODO : Remove: This is LLM specific, move to agnostic format.
    gen_kwargs: Optional[str] = None
    bootstrap_iters: Optional[int] = 100000
    qty: int
    tasks: str
    total_instances: int
    request_type: str  # TODO : Remove, specific of LMEH
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    done: bool = False
    evaluated: bool = False
    drop: bool = False

    class Config:
        populate_by_name = True
        json_schema_extra = {"example": {"_id": "60d3216d82e029466c6811d2"}}


###########
# EVALUATOR
###########


# TODO: Sepparate this class into an agnostic input to the evaluation workflow.
# This class is inhering multiple optional parameters that dont play any role in
# non-LMEH or non-LLM tasks.
class PocketNetworkEvaluationTaskRequest(PocketNetworkTaskRequest):
    framework: Optional[str] = None
    task_id: Union[str, PyObjectId]
    tasks: Optional[str] = None
    requester_args: Optional[RequesterArgs] = None

    @field_validator("qty")
    def check_qty(cls, v):
        return v

    @model_validator(mode="after")
    def verify_qty_or_doc_ids(self):
        return self

    @model_validator(mode="after")
    def remove_blacklist_when_all(self):
        return self


# From vllm/entrypoints/openai/protocol.py
class UsageInfo(OpenAIBaseModel):
    prompt_tokens: int = 0
    total_tokens: int = 0
    completion_tokens: Optional[int] = 0


class CompletionLogProbs(OpenAIBaseModel):
    text_offset: List[int] = Field(default_factory=list)
    token_logprobs: List[Optional[float]] = Field(default_factory=list)
    tokens: List[str] = Field(default_factory=list)
    top_logprobs: Optional[List[Optional[Dict[str, float]]]] = None


class CompletionResponseChoice(OpenAIBaseModel):
    index: int
    text: str
    logprobs: Optional[CompletionLogProbs] = None
    finish_reason: Optional[str] = None
    stop_reason: Optional[Union[int, str]] = Field(
        default=None,
        description=(
            "The stop string or token id that caused the completion "
            "to stop, None if the completion finished for some other reason "
            "including encountering the EOS token"
        ),
    )


class CompletionResponse(OpenAIBaseModel):
    id: str = Field(default_factory=lambda: f"cmpl-{str(uuid.uuid4().hex)}")
    object: str = "text_completion"
    created: int = Field(default_factory=lambda: int(time.time()))
    model: str
    choices: List[CompletionResponseChoice]
    usage: UsageInfo
    response_time: int  # Total time to complete request (POKT Network)


###########
# RESPONSES
###########


class PocketNetworkMongoDBResultBase(BaseModel):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    task_id: ObjectId
    num_samples: int
    status: int
    result_height: int
    result_time: datetime

    class Config:
        arbitrary_types_allowed = True


class SignatureSample(BaseModel):
    signature: str
    id: int
    status_code: int


class PocketNetworkMongoDBResultSignature(BaseModel):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    result_data: PocketNetworkMongoDBResultBase
    signatures: List[SignatureSample]

    class Config:
        arbitrary_types_allowed = True


class NumericSample(BaseModel):
    score: float
    id: int
    run_time: float
    status_code: int


class PocketNetworkMongoDBResultNumerical(BaseModel):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    result_data: PocketNetworkMongoDBResultBase
    scores: List[NumericSample]

    class Config:
        arbitrary_types_allowed = True


###########
# Tokenizer
###########


class PocketNetworkMongoDBTokenizer(BaseModel):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    tokenizer: dict
    hash: str

    class Config:
        arbitrary_types_allowed = True


class PocketNetworkMongoDBConfig(BaseModel):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    config: dict
    hash: str

    class Config:
        arbitrary_types_allowed = True


######################
# TIMEOUT HANDLER
######################


class TTFT(BaseModel):
    prompt_lenght: List[int]
    sla_time: List[int]


class LLMTimeouts(BaseModel):
    ttft: TTFT
    tpot: float
    queue: float
    type: str = "llm"


class TimeoutHandler(BaseModel):
    model_config = ConfigDict(extra="allow")
    timeouts: Optional[LLMTimeouts] = None

    def llm_timeout(self, prefill: int, decode: int) -> float:
        timeout = self.ttft(prefill) + (self.tpot * decode) + self.queue
        return float(timeout)

    # whenever a new timeout type is added, add a new function here

    def chain_default(self, prefill: int, decode: int) -> float:
        return 60

    @model_validator(mode="after")
    def map_timeouts(self):
        if self.timeouts:
            chain_timeouts: Dict[str, Callable[[], str]] = {
                "llm": self.llm_timeout,
            }
            self._timeout_fn = chain_timeouts.get(
                self.timeouts.type, self.chain_default
            )
        else:
            # if timeouts are not defined, means default
            self._timeout_fn = self.chain_default
        return

    def model_post_init(self, __context: Any) -> None:
        if self.timeouts is None:
            # if timeouts are not defined, means default
            return
        if self.timeouts.type == "llm":
            try:
                import numpy as np

                x = self.timeouts.ttft.prompt_lenght
                y = self.timeouts.ttft.sla_time
                self.queue = self.timeouts.queue
                z = np.polyfit(x, y, 2)
                self.ttft = np.poly1d(z)
                self.tpot = self.timeouts.tpot
            except Exception as e:
                raise ValueError(f"Error creating timeout function: {e}")
        # whenever a new timeout type is added, add new post init here
        # to define attributes.
        return

    def get_timeout(self, **kwargs) -> float:
        return self._timeout_fn(**kwargs)
