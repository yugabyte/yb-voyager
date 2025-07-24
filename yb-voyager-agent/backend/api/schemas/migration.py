from pydantic import BaseModel, Field
from typing import Optional
from datetime import datetime
from models.migration import MigrationStatus, MigrationPhase

class MigrationBase(BaseModel):
    name: str = Field(..., min_length=1, max_length=255)
    description: Optional[str] = None
    source_type: Optional[str] = None
    source_host: Optional[str] = None
    source_port: Optional[int] = None
    source_database: Optional[str] = None
    target_host: Optional[str] = None
    target_port: Optional[int] = None
    target_database: Optional[str] = None
    config: Optional[str] = None

class MigrationCreate(MigrationBase):
    config_content: Optional[str] = None
    config_option: Optional[str] = None  # 'use_existing', 'create_new', 'ai_guided'
    config_file_path: Optional[str] = None

class MigrationUpdate(BaseModel):
    name: Optional[str] = Field(None, min_length=1, max_length=255)
    description: Optional[str] = None
    status: Optional[MigrationStatus] = None
    phase: Optional[MigrationPhase] = None
    source_type: Optional[str] = None
    source_host: Optional[str] = None
    source_port: Optional[int] = None
    source_database: Optional[str] = None
    target_host: Optional[str] = None
    target_port: Optional[int] = None
    target_database: Optional[str] = None
    config: Optional[str] = None
    progress_percentage: Optional[int] = Field(None, ge=0, le=100)
    current_step: Optional[str] = None
    error_message: Optional[str] = None

class MigrationResponse(MigrationBase):
    id: int
    status: MigrationStatus
    phase: MigrationPhase
    progress_percentage: int
    current_step: Optional[str] = None
    error_message: Optional[str] = None
    config_option: Optional[str] = None
    config_file_path: Optional[str] = None
    created_at: datetime
    updated_at: Optional[datetime] = None
    started_at: Optional[datetime] = None
    completed_at: Optional[datetime] = None

    class Config:
        from_attributes = True

class MigrationListResponse(BaseModel):
    id: int
    name: str
    status: MigrationStatus
    phase: MigrationPhase
    progress_percentage: int
    config_option: Optional[str] = None
    config_file_path: Optional[str] = None
    created_at: datetime
    updated_at: Optional[datetime] = None

    class Config:
        from_attributes = True 
