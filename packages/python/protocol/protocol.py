import time
import uuid
from datetime import datetime
from typing import Any, ClassVar, Callable, Dict, List, Literal, Optional, Union
from collections.abc import Iterable
from dataclasses import dataclass

from bson import ObjectId
from openai.types.chat import ChatCompletionContentPartInputAudioParam
from openai.types.chat import (
    ChatCompletionContentPartParam as OpenAIChatCompletionContentPartParam,
)
from openai.types.chat import ChatCompletionContentPartRefusalParam
from openai.types.chat import (
    ChatCompletionMessageParam as OpenAIChatCompletionMessageParam,
)
from openai.types.chat import ChatCompletionMessageToolCallParam

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

# pydantic needs the TypedDict from typing_extensions
from typing_extensions import Required, TypeAlias, TypedDict

_LONG_INFO_min = -9223372036854775808  # from torch.iinfo(torch.long).min,
_LONG_INFO_max = 9223372036854775807  # from torch.iinfo(torch.long).max)


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
    path: str = "/v1/chat/completions"
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
    gen_kwargs: Optional[Union[str, dict]] = None
    bootstrap_iters: Optional[int] = 100000
    system_instruction: Optional[str] = None
    apply_chat_template: Optional[bool] = True
    fewshot_as_multiturn: Optional[bool] = True
    confirm_run_unsafe_code: Optional[bool] = False

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

    @model_validator(mode="after")
    # "When `fewshot_as_multiturn` is selected, `apply_chat_template` must be set (either to `True` or to the chosen template name)."
    def verify_fewshot_as_multiturn(self):
        if self.fewshot_as_multiturn and self.apply_chat_template is False:
            raise ValueError(
                "When `fewshot_as_multiturn` is selected, `apply_chat_template` must be set (either to `True` or to the chosen template name)."
            )
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
    model_config = ConfigDict(extra="allow")


class JsonSchemaResponseFormat(OpenAIBaseModel):
    name: str
    description: Optional[str] = None
    # schema is the field in openai but that causes conflicts with pydantic so
    # instead use json_schema with an alias
    json_schema: Optional[dict[str, Any]] = Field(default=None, alias="schema")
    strict: Optional[bool] = None


class StructuralTag(OpenAIBaseModel):
    begin: str
    # schema is the field, but that causes conflicts with pydantic so
    # instead use structural_tag_schema with an alias
    structural_tag_schema: Optional[dict[str, Any]] = Field(
        default=None, alias="schema"
    )
    end: str


class StructuralTagResponseFormat(OpenAIBaseModel):
    type: Literal["structural_tag"]
    structures: list[StructuralTag]
    triggers: list[str]


class ResponseFormat(OpenAIBaseModel):
    # type must be "json_schema", "json_object", or "text"
    type: Literal["text", "json_object", "json_schema"]
    json_schema: Optional[JsonSchemaResponseFormat] = None


AnyResponseFormat = Union[ResponseFormat, StructuralTagResponseFormat]


class StreamOptions(OpenAIBaseModel):
    include_usage: Optional[bool] = True
    continuous_usage_stats: Optional[bool] = False


class FunctionDefinition(OpenAIBaseModel):
    name: str
    description: Optional[str] = None
    parameters: Optional[dict[str, Any]] = None


class ChatCompletionToolsParam(OpenAIBaseModel):
    type: Literal["function"] = "function"
    function: FunctionDefinition


class ChatCompletionNamedFunction(OpenAIBaseModel):
    name: str


class ChatCompletionNamedToolChoiceParam(OpenAIBaseModel):
    function: ChatCompletionNamedFunction
    type: Literal["function"] = "function"


class CompletionRequest(BaseModel):
    # Ordered by official OpenAI API documentation
    # https://platform.openai.com/docs/api-reference/completions/create
    model: str
    prompt: Optional[Union[list[int], list[list[int]], str, list[str]]]
    prompt_embeds: Optional[Union[bytes, list[bytes]]] = None
    best_of: Optional[int] = None
    echo: Optional[bool] = False
    frequency_penalty: Optional[float] = 0.0
    logit_bias: Optional[dict[str, float]] = None
    logprobs: Optional[int] = None
    max_tokens: Optional[int] = 16
    n: int = 1
    presence_penalty: Optional[float] = 0.0
    seed: Optional[int] = Field(None, ge=_LONG_INFO_min, le=_LONG_INFO_max)
    stop: Optional[Union[str, list[str]]] = []
    stream: Optional[bool] = False
    stream_options: Optional[StreamOptions] = None
    suffix: Optional[str] = None
    temperature: Optional[float] = None
    top_p: Optional[float] = None
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

    def estimate_ctxlen(self) -> int:
        assert isinstance(
            self.prompt, (str, list)
        ), "Prompt must be a string or a list of strings or integers"
        if isinstance(self.prompt, str):
            self.ctxlen = int(len(self.prompt.split(" ")) / 0.75)
        return


#########################
# Chat Completion Request
#########################

# Subclases from vllm repo: vllm/entrypoints/chat_utils.py


class AudioURL(TypedDict, total=False):
    url: Required[str]
    """
    Either a URL of the audio or a data URL with base64 encoded audio data.
    """


class ChatCompletionContentPartAudioParam(TypedDict, total=False):
    audio_url: Required[AudioURL]

    type: Required[Literal["audio_url"]]
    """The type of the content part."""


class ChatCompletionContentPartImageEmbedsParam(TypedDict, total=False):
    image_embeds: Required[Union[str, dict[str, str]]]
    """
    The image embeddings. It can be either:
    - A single base64 string.
    - A dictionary where each value is a base64 string.
    """
    type: Required[Literal["image_embeds"]]
    """The type of the content part."""


class VideoURL(TypedDict, total=False):
    url: Required[str]
    """
    Either a URL of the video or a data URL with base64 encoded video data.
    """


class ChatCompletionContentPartVideoParam(TypedDict, total=False):
    video_url: Required[VideoURL]

    type: Required[Literal["video_url"]]
    """The type of the content part."""


class CustomChatCompletionContentSimpleImageParam(TypedDict, total=False):
    """A simpler version of the param that only accepts a plain image_url.
    This is supported by OpenAI API, although it is not documented.

    Example:
    {
        "image_url": "https://example.com/image.jpg"
    }
    """

    image_url: Required[str]


class CustomChatCompletionContentSimpleAudioParam(TypedDict, total=False):
    """A simpler version of the param that only accepts a plain audio_url.

    Example:
    {
        "audio_url": "https://example.com/audio.mp3"
    }
    """

    audio_url: Required[str]


class CustomChatCompletionContentSimpleVideoParam(TypedDict, total=False):
    """A simpler version of the param that only accepts a plain audio_url.

    Example:
    {
        "video_url": "https://example.com/video.mp4"
    }
    """

    video_url: Required[str]


ChatCompletionContentPartParam: TypeAlias = Union[
    OpenAIChatCompletionContentPartParam,
    ChatCompletionContentPartAudioParam,
    ChatCompletionContentPartInputAudioParam,
    ChatCompletionContentPartVideoParam,
    ChatCompletionContentPartRefusalParam,
    CustomChatCompletionContentSimpleImageParam,
    ChatCompletionContentPartImageEmbedsParam,
    CustomChatCompletionContentSimpleAudioParam,
    CustomChatCompletionContentSimpleVideoParam,
    str,
]


class CustomChatCompletionMessageParam(TypedDict, total=False):
    """Enables custom roles in the Chat Completion API."""

    role: Required[str]
    """The role of the message's author."""

    content: Union[str, list[ChatCompletionContentPartParam]]
    """The contents of the message."""

    name: str
    """An optional name for the participant.

    Provides the model information to differentiate between participants of the
    same role.
    """

    tool_call_id: Optional[str]
    """Tool call that this message is responding to."""

    tool_calls: Optional[Iterable[ChatCompletionMessageToolCallParam]]
    """The tool calls generated by the model, such as function calls."""


ChatCompletionMessageParam = Union[
    OpenAIChatCompletionMessageParam, CustomChatCompletionMessageParam
]


class ChatCompletionRequest(OpenAIBaseModel):
    # Ordered by official OpenAI API documentation
    # https://platform.openai.com/docs/api-reference/chat/create
    messages: list[ChatCompletionMessageParam]
    model: Optional[str] = None
    frequency_penalty: Optional[float] = 0.0
    logit_bias: Optional[dict[str, float]] = None
    logprobs: Optional[bool] = False
    top_logprobs: Optional[int] = 0
    # TODO(#9845): remove max_tokens when field is removed from OpenAI API
    max_tokens: Optional[int] = Field(
        default=None,
        deprecated="max_tokens is deprecated in favor of the max_completion_tokens field",
    )
    max_completion_tokens: Optional[int] = None
    n: Optional[int] = 1
    presence_penalty: Optional[float] = 0.0
    response_format: Optional[AnyResponseFormat] = None
    seed: Optional[int] = Field(None, ge=_LONG_INFO_min, le=_LONG_INFO_max)
    stop: Optional[Union[str, list[str]]] = []
    stream: Optional[bool] = False
    stream_options: Optional[StreamOptions] = None
    temperature: Optional[float] = None
    top_p: Optional[float] = None
    tools: Optional[list[ChatCompletionToolsParam]] = None
    tool_choice: Optional[
        Union[
            Literal["none"],
            Literal["auto"],
            Literal["required"],
            ChatCompletionNamedToolChoiceParam,
        ]
    ] = "none"

    # NOTE this will be ignored by vLLM -- the model determines the behavior
    parallel_tool_calls: Optional[bool] = False
    user: Optional[str] = None
    # --8<-- [start:chat-completion-sampling-params]
    # NOTE (nicolas) Do not considered.
    # --8<-- [end:chat-completion-sampling-params]

    # NOTE (nicolas) Fields to avoid futures tokenizer's call
    # ctxlen, context_enc & continuation_enc are specific for
    # the pocket-ml-testbench
    ctxlen: Optional[int] = None
    context_enc: Optional[List[int]] = None
    continuation_enc: Optional[List[int]] = None

    def estimate_ctxlen(self) -> int:
        """
        Estimate the context length of chat completion messages.

        Args:
            messages: List of chat completion message parameters

        Returns:
            Estimated total context length in tokens
        """
        total_tokens = 0

        for message in self.messages:
            content = message.get("content", "")
            if not isinstance(content, str):
                # Handle different content types with errors
                raise ValueError("Content must be a string or a list of content parts")
            else:
                # Simple string content
                tokens = int(len(content.split(" ")) / 0.75) + 2
            total_tokens += tokens
        self.ctxlen = total_tokens
        return


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
    data: Union[str, CompletionRequest, ChatCompletionRequest]
    task_id: ObjectId
    instance_id: ObjectId
    timeout: int = 0
    done: bool = False
    trigger_session: int = 0  # This is filled by the relayer when triggered
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
    gen_kwargs: Optional[Union[str, dict]] = None
    bootstrap_iters: Optional[int] = 100000
    system_instruction: Optional[str] = None
    apply_chat_template: Optional[bool] = True
    fewshot_as_multiturn: Optional[bool] = True
    confirm_run_unsafe_code: Optional[bool] = False
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


# TODO: Separate this class into an agnostic input to the evaluation workflow.
# This class is inhering multiple optional parameters that don't play any role in
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
    prompt_logprobs: Optional[Any] = None  # TODO : Set correct class
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


# Chat Completion Response Classes


# We use dataclass for now because it is used for
# openai server output, and msgspec is not serializable.
# TODO(sang): Fix it.
@dataclass
class Logprob:
    """Infos for supporting OpenAI compatible logprobs and token ranks.

    Attributes:
        logprob: The logprob of chosen token
        rank: The vocab rank of chosen token (>=1)
        decoded_token: The decoded chosen token index
    """

    logprob: float
    rank: Optional[int] = None
    decoded_token: Optional[str] = None


class FunctionCall(OpenAIBaseModel):
    name: str
    arguments: str


class ToolCall(OpenAIBaseModel):
    id: str
    type: Literal["function"] = "function"
    function: FunctionCall


class DeltaFunctionCall(BaseModel):
    name: Optional[str] = None
    arguments: Optional[str] = None


# a tool call delta where everything is optional
class DeltaToolCall(OpenAIBaseModel):
    id: Optional[str] = None
    type: Optional[Literal["function"]] = None
    index: int
    function: Optional[DeltaFunctionCall] = None


class ExtractedToolCallInformation(BaseModel):
    # indicate if tools were called
    tools_called: bool

    # extracted tool calls
    tool_calls: list[ToolCall]

    # content - per OpenAI spec, content AND tool calls can be returned rarely
    # But some models will do this intentionally
    content: Optional[str] = None


class ChatMessage(OpenAIBaseModel):
    role: str
    reasoning_content: Optional[str] = None
    content: Optional[str] = None
    tool_calls: list[ToolCall] = Field(default_factory=list)


class ChatCompletionLogProb(OpenAIBaseModel):
    token: str
    logprob: float = -9999.0
    bytes: Optional[list[int]] = None


class ChatCompletionLogProbsContent(ChatCompletionLogProb):
    # Workaround: redefine fields name cache so that it's not
    # shared with the super class.
    field_names: ClassVar[Optional[set[str]]] = None
    top_logprobs: list[ChatCompletionLogProb] = Field(default_factory=list)


class ChatCompletionLogProbs(OpenAIBaseModel):
    content: Optional[list[ChatCompletionLogProbsContent]] = None


class ChatCompletionResponseChoice(OpenAIBaseModel):
    index: int
    message: ChatMessage
    logprobs: Optional[ChatCompletionLogProbs] = None
    # per OpenAI spec this is the default
    finish_reason: Optional[str] = "stop"
    # not part of the OpenAI spec but included in vLLM for legacy reasons
    stop_reason: Optional[Union[int, str]] = None


class ChatCompletionResponse(OpenAIBaseModel):
    id: str
    object: Literal["chat.completion"] = "chat.completion"
    created: int = Field(default_factory=lambda: int(time.time()))
    model: str
    choices: list[ChatCompletionResponseChoice]
    usage: UsageInfo
    prompt_logprobs: Optional[list[Optional[dict[int, Logprob]]]] = None


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
    error_str: str


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
    error_str: str


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


###########
# TAXONOMY SUMMARIZER
###########


class PocketNetworkTaxonomySummaryTaskRequest(BaseModel):
    supplier_id: Union[str, PyObjectId]
    taxonomy: str


class TaxonomyNodeSummary(BaseModel):
    score: float
    score_dev: float
    run_time: float
    run_time_dev: float
    sample_min: int

    # TODO : Extend this class to compute running means from passing a series of numerical buffers


class PocketNetworkMongoDBTaxonomySummary(BaseModel):
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    supplier_id: ObjectId
    summary_date: datetime
    taxonomy_name: str
    taxonomy_nodes_scores: Dict[str, TaxonomyNodeSummary]

    class Config:
        arbitrary_types_allowed = True
