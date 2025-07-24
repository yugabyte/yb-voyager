from sqlalchemy import Column, Integer, String, DateTime, Text, ForeignKey, Enum
from sqlalchemy.sql import func
from sqlalchemy.orm import relationship
from core.database import Base
import enum

class MessageType(str, enum.Enum):
    USER = "user"
    AGENT = "agent"
    SYSTEM = "system"

class Message(Base):
    __tablename__ = "messages"
    
    id = Column(Integer, primary_key=True, index=True)
    migration_id = Column(Integer, ForeignKey("migrations.id"), nullable=True)
    session_id = Column(String(255), nullable=False, index=True)
    
    # Message content
    type = Column(Enum(MessageType), nullable=False)
    content = Column(Text, nullable=False)
    
    # Metadata
    timestamp = Column(DateTime(timezone=True), server_default=func.now())
    message_metadata = Column(Text, nullable=True)  # JSON string for additional data
    
    # Relationships
    migration = relationship("Migration", back_populates="messages")
    
    def __repr__(self):
        return f"<Message(id={self.id}, type='{self.type}', session='{self.session_id}')>" 
