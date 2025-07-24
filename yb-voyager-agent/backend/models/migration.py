from sqlalchemy import Column, Integer, String, DateTime, Text, Enum
from sqlalchemy.sql import func
from sqlalchemy.orm import relationship
from core.database import Base
import enum

class MigrationStatus(str, enum.Enum):
    IDLE = "idle"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"

class MigrationPhase(str, enum.Enum):
    ASSESSMENT = "assessment"
    SCHEMA = "schema"
    DATA = "data"
    VALIDATION = "validation"

class Migration(Base):
    __tablename__ = "migrations"
    
    id = Column(Integer, primary_key=True, index=True)
    name = Column(String(255), nullable=False)
    description = Column(Text, nullable=True)
    status = Column(Enum(MigrationStatus), default=MigrationStatus.IDLE)
    phase = Column(Enum(MigrationPhase), default=MigrationPhase.ASSESSMENT)
    
    # Source and target database info
    source_type = Column(String(50), nullable=True)  # postgresql, mysql, etc.
    source_host = Column(String(255), nullable=True)
    source_port = Column(Integer, nullable=True)
    source_database = Column(String(255), nullable=True)
    
    target_host = Column(String(255), nullable=True)
    target_port = Column(Integer, nullable=True)
    target_database = Column(String(255), nullable=True)
    
    # Configuration
    config = Column(Text, nullable=True)  # JSON string for migration config
    config_option = Column(String(50), nullable=True)  # 'use_existing', 'ai_guided'
    config_file_path = Column(String(500), nullable=True)  # Path to config file on disk
    
    # Timestamps
    created_at = Column(DateTime(timezone=True), server_default=func.now())
    updated_at = Column(DateTime(timezone=True), onupdate=func.now())
    started_at = Column(DateTime(timezone=True), nullable=True)
    completed_at = Column(DateTime(timezone=True), nullable=True)
    
    # Progress tracking
    progress_percentage = Column(Integer, default=0)
    current_step = Column(String(255), nullable=True)
    error_message = Column(Text, nullable=True)
    
    # Relationships
    messages = relationship("Message", back_populates="migration")
    
    def __repr__(self):
        return f"<Migration(id={self.id}, name='{self.name}', status='{self.status}')>" 
