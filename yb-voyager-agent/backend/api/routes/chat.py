from fastapi import APIRouter, WebSocket, WebSocketDisconnect, Depends, HTTPException
from sqlalchemy.orm import Session
from typing import List, Dict
import json
import logging
from datetime import datetime

from core.database import get_db
from models.message import Message, MessageType
from models.migration import Migration
from api.schemas.chat import ChatMessage, ChatResponse

logger = logging.getLogger(__name__)

router = APIRouter()

# Store active WebSocket connections
class ConnectionManager:
    def __init__(self):
        self.active_connections: Dict[str, WebSocket] = {}

    async def connect(self, websocket: WebSocket, session_id: str):
        await websocket.accept()
        self.active_connections[session_id] = websocket
        logger.info(f"WebSocket connected for session: {session_id}")

    def disconnect(self, session_id: str):
        if session_id in self.active_connections:
            del self.active_connections[session_id]
            logger.info(f"WebSocket disconnected for session: {session_id}")

    async def send_personal_message(self, message: str, session_id: str):
        if session_id in self.active_connections:
            await self.active_connections[session_id].send_text(message)

manager = ConnectionManager()

@router.websocket("/ws/{session_id}")
async def websocket_endpoint(websocket: WebSocket, session_id: str):
    await manager.connect(websocket, session_id)
    try:
        while True:
            data = await websocket.receive_text()
            message_data = json.loads(data)
            
            # Process the message
            response = await process_chat_message(message_data, session_id)
            
            # Send response back
            await manager.send_personal_message(json.dumps(response), session_id)
            
    except WebSocketDisconnect:
        manager.disconnect(session_id)

@router.post("/message", response_model=ChatResponse)
async def send_message(
    message: ChatMessage,
    db: Session = Depends(get_db)
):
    """Send a message via REST API (for non-WebSocket clients)"""
    response = await process_chat_message(message.dict(), message.session_id, db)
    return response

@router.get("/history/{session_id}")
async def get_chat_history(
    session_id: str,
    limit: int = 50,
    db: Session = Depends(get_db)
):
    """Get chat history for a session"""
    messages = db.query(Message).filter(
        Message.session_id == session_id
    ).order_by(Message.timestamp.desc()).limit(limit).all()
    
    return {
        "session_id": session_id,
        "messages": [
            {
                "id": msg.id,
                "type": msg.type,
                "content": msg.content,
                "timestamp": msg.timestamp
            }
            for msg in reversed(messages)  # Return in chronological order
        ]
    }

async def process_chat_message(message_data: dict, session_id: str, db: Session = None):
    """Process incoming chat message and generate response"""
    try:
        content = message_data.get("content", "")
        migration_id = message_data.get("migration_id")
        
        # Try to use AI framework first
        try:
            from api.routes.ai_framework import ai_framework, AIFrameworkRequest
            
            # Create AI framework request - always goes to supervisor
            ai_request = AIFrameworkRequest(
                message=content,
                session_id=session_id,
                migration_id=migration_id,
                agent_type="supervisor"  # Always supervisor - it handles internal routing
            )
            
            # Process through AI framework
            ai_response = ai_framework.process_message(ai_request)
            
            # Save user message to database
            if db:
                user_message = Message(
                    session_id=session_id,
                    migration_id=migration_id,
                    type=MessageType.USER,
                    content=content
                )
                db.add(user_message)
                db.commit()
            
            # Save AI response to database
            if db:
                agent_message = Message(
                    session_id=session_id,
                    migration_id=migration_id,
                    type=MessageType.AGENT,
                    content=ai_response.agent_response
                )
                db.add(agent_message)
                db.commit()
            
            return {
                "type": "agent",
                "content": ai_response.agent_response,
                "timestamp": ai_response.timestamp,
                "session_id": session_id,
                "agent_type": ai_response.agent_type,
                "metadata": ai_response.metadata
            }
            
        except ImportError:
            logger.warning("AI Framework not available, falling back to basic response")
            # Fallback to basic response if AI framework is not available
            pass
        
        # Fallback: Generate basic agent response
        agent_response = generate_agent_response(content, migration_id)
        
        # Save user message to database
        if db:
            user_message = Message(
                session_id=session_id,
                migration_id=migration_id,
                type=MessageType.USER,
                content=content
            )
            db.add(user_message)
            db.commit()
        
        # Save agent message to database
        if db:
            agent_message = Message(
                session_id=session_id,
                migration_id=migration_id,
                type=MessageType.AGENT,
                content=agent_response
            )
            db.add(agent_message)
            db.commit()
        
        return {
            "type": "agent",
            "content": agent_response,
            "timestamp": datetime.utcnow().isoformat(),
            "session_id": session_id
        }
        
    except Exception as e:
        logger.error(f"Error processing chat message: {e}")
        return {
            "type": "system",
            "content": "Sorry, I encountered an error processing your message.",
            "timestamp": datetime.utcnow().isoformat(),
            "session_id": session_id
        }

def generate_agent_response(user_message: str, migration_id: int = None) -> str:
    """Generate response from the migration agent"""
    # This is a placeholder - will be replaced with actual agent logic
    if "hello" in user_message.lower() or "hi" in user_message.lower():
        return "Hello! I'm your YB Voyager migration assistant. How can I help you today?"
    
    elif "migration" in user_message.lower():
        return "I can help you with database migrations. Would you like to start a new migration project or check the status of existing ones?"
    
    elif "schema" in user_message.lower():
        return "Schema migration is a key part of the process. I can help you analyze your source database schema and prepare it for migration to YB."
    
    else:
        return "I understand you're interested in database migration. I can help you with assessment, schema migration, data migration, and validation. What specific aspect would you like to work on?" 
