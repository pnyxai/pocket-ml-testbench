#import torch
from typing import List, Literal, Optional, Union, Dict
from bson import ObjectId
from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

class PocketNetworkRegisterTaskRequest(BaseModel):
    framework: Literal["lmeh", "helm"]
    tasks: str
    verbosity: Optional[Literal["CRITICAL", "ERROR", "WARNING", "INFO", "DEBUG"]] = "INFO"
    include_path: Optional[str] = None
    postgres_uri: Optional[str] = None
    mongodb_uri: Optional[str] = None

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
    llm_args: Optional[Dict] = None
    model: Optional[str] = "pocket_network"

    @field_validator("qty")
    def check_qty(cls, v):
        if v <= 0:
            raise ValueError("qty must be greater than 0")
        return v

    @model_validator(mode="after")
    def verify_qty_or_doc_ids(self):
        if (self.qty and self.doc_ids) or (not self.qty and not self.doc_ids):
            raise ValueError("Expected qty or doc_ids but not both.")
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
            raise ValueError('Not a valid ObjectId')
        return str(v)

class PocketNetworkMongoDBTask(BaseModel):
    framework: Literal["lmeh", "helm"]
    requester_args: RequesterArgs
    blacklist: Optional[List[int]] = []
    qty: int
    tasks: str
    total_instances: int
    request_type: str
    id: PyObjectId = Field(default_factory=PyObjectId, alias="_id")
    done: bool = False

    class Config:
        allow_population_by_field_name = True
        schema_extra = {
            "example": {
                "_id": "60d3216d82e029466c6811d2"
            }
        }


### From vllm/entrypoints/openai/protocol.py
class OpenAIBaseModel(BaseModel):
    # OpenAI API does not allow extra fields
    model_config = ConfigDict(extra="forbid")

class CompletionRequest(OpenAIBaseModel):
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
    seed: Optional[int] = Field(None,
                                ge=-9223372036854775808, #from torch.iinfo(torch.long).min,
                                le=9223372036854775807) #from torch.iinfo(torch.long).max)
    stop: Optional[Union[str, List[str]]] = Field(default_factory=list)
    stream: Optional[bool] = False
    suffix: Optional[str] = None
    temperature: Optional[float] = 1.0
    top_p: Optional[float] = 1.0
    user: Optional[str] = None

    def to_dict(self, remove_fields: Optional[List[str]] = None):
        data = self.model_dump(exclude_defaults=True)
        if remove_fields:
            for field in remove_fields:
                if field in data:
                    del data[field]
        return data

class PromptMongoDB(BaseModel):
    model_config = ConfigDict(arbitrary_types_allowed=True)
    _id: Optional[ObjectId] = None
    data: str
    task_id: ObjectId
    instance_id: ObjectId
    timeout: int = 20
    done: bool = False

    @model_validator(mode="after")
    def create_id(cls, values):
        if "_id" not in values:
            values._id = ObjectId()
        return values