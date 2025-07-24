from pydantic import BaseModel, Field
from typing import Optional
from datetime import datetime

class ChatMessage(BaseModel):
    content: str = Field(..., min_length=1)
    session_id: str = Field(..., min_length=1)
    migration_id: Optional[int] = None
    message_type: Optional[str] = "user"

class ChatResponse(BaseModel):
    type: str
    content: str
    timestamp: str
    session_id: str

class ChatHistoryResponse(BaseModel):
    session_id: str
    messages: list

class WebSocketMessage(BaseModel):
    type: str
    content: str
    migration_id: Optional[int] = None 
