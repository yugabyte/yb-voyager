from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel
from typing import Optional, Dict, Any, List
import json
import os
import logging
from datetime import datetime

from core.database import get_db
from models.message import Message, MessageType
from api.routes.settings import load_settings

logger = logging.getLogger(__name__)

router = APIRouter()

# AI Framework Integration Models
class AIFrameworkRequest(BaseModel):
    """Request model for AI framework interactions"""
    message: str
    session_id: str
    migration_id: Optional[int] = None
    agent_type: Optional[str] = "supervisor"  # supervisor, config, schema, data, validation
    context: Optional[Dict[str, Any]] = None

class AIFrameworkResponse(BaseModel):
    """Response model for AI framework interactions"""
    success: bool
    message: str
    agent_response: str
    agent_type: str
    metadata: Optional[Dict[str, Any]] = None
    timestamp: str

class AgentStatus(BaseModel):
    """Status model for agent availability"""
    supervisor_available: bool
    config_agent_available: bool
    schema_agent_available: bool
    data_agent_available: bool
    validation_agent_available: bool
    active_agent: Optional[str] = None

class ConfigAgentRequest(BaseModel):
    """Specific request model for config agent"""
    action: str  # create, validate, modify, help
    config_file_path: Optional[str] = None
    config_content: Optional[str] = None
    template_type: Optional[str] = None
    parameters: Optional[Dict[str, Any]] = None

# Mock AI Framework Integration (will be replaced with actual framework)
class MockAIFramework:
    """Mock AI framework for testing - will be replaced with actual implementation"""
    
    def __init__(self):
        self.settings = {}
        self.active_sessions = {}
    
    def initialize_with_settings(self, settings: Dict[str, Any]):
        """Initialize the AI framework with UI settings"""
        self.settings = settings
        logger.info(f"AI Framework initialized with model: {settings.get('model', 'unknown')}")
        return True
    
    def get_agent_status(self) -> AgentStatus:
        """Get status of all agents"""
        return AgentStatus(
            supervisor_available=True,
            config_agent_available=True,
            schema_agent_available=False,  # Not implemented yet
            data_agent_available=False,    # Not implemented yet
            validation_agent_available=False,  # Not implemented yet
            active_agent="supervisor"
        )
    
    def process_message(self, request: AIFrameworkRequest) -> AIFrameworkResponse:
        """Process a message through the AI framework"""
        try:
            # Load current settings
            settings = load_settings()
            
            # Initialize framework if not already done
            if not self.settings:
                self.initialize_with_settings(settings)
            
            # Always start with supervisor - it handles internal routing
            if request.agent_type == "supervisor":
                return self._handle_supervisor_agent(request, settings)
            else:
                # If somehow we get a direct agent request, route through supervisor
                logger.warning(f"Direct agent request received for {request.agent_type}, routing through supervisor")
                return self._handle_supervisor_agent(request, settings)
                
        except Exception as e:
            logger.error(f"Error processing message: {e}")
            return AIFrameworkResponse(
                success=False,
                message="Error processing request",
                agent_response=f"Sorry, I encountered an error: {str(e)}",
                agent_type="supervisor",
                timestamp=datetime.utcnow().isoformat()
            )
    
    def _handle_supervisor_agent(self, request: AIFrameworkRequest, settings: Dict[str, Any]) -> AIFrameworkResponse:
        """Handle supervisor agent routing and coordination"""
        message_lower = request.message.lower()
        
        # Check for config-related requests
        if any(keyword in message_lower for keyword in ["config", "configuration", "yaml", "template", "create config", "validate config", "modify config"]):
            # Route to config agent and get response
            config_response = self._handle_config_agent(request, settings)
            
            return AIFrameworkResponse(
                success=True,
                message="Supervisor coordinated with config agent",
                agent_response=config_response.agent_response,
                agent_type="supervisor",
                metadata={
                    "routed_to": "config_agent",
                    "config_agent_response": config_response.agent_response,
                    "available_templates": ["offline-migration", "live-migration", "bulk-data-load"]
                },
                timestamp=datetime.utcnow().isoformat()
            )
        
        # Check for schema-related requests
        elif any(keyword in message_lower for keyword in ["schema", "table", "structure", "export schema", "import schema"]):
            return AIFrameworkResponse(
                success=True,
                message="Supervisor response - schema agent not yet implemented",
                agent_response="I understand you need help with schema operations. The schema agent is not yet implemented, but I can help you with configuration files for now.",
                agent_type="supervisor",
                metadata={"note": "Schema agent coming soon"},
                timestamp=datetime.utcnow().isoformat()
            )
        
        # Check for data migration requests
        elif any(keyword in message_lower for keyword in ["data", "migrate data", "data migration", "bulk load"]):
            return AIFrameworkResponse(
                success=True,
                message="Supervisor response - data agent not yet implemented",
                agent_response="I understand you need help with data migration. The data agent is not yet implemented, but I can help you with configuration files for now.",
                agent_type="supervisor",
                metadata={"note": "Data agent coming soon"},
                timestamp=datetime.utcnow().isoformat()
            )
        
        # Check for validation requests
        elif any(keyword in message_lower for keyword in ["validate", "validation", "check", "verify"]):
            return AIFrameworkResponse(
                success=True,
                message="Supervisor response - validation agent not yet implemented",
                agent_response="I understand you need help with validation. The validation agent is not yet implemented, but I can help you with configuration files for now.",
                agent_type="supervisor",
                metadata={"note": "Validation agent coming soon"},
                timestamp=datetime.utcnow().isoformat()
            )
        
        # General migration help
        elif any(keyword in message_lower for keyword in ["migration", "migrate", "voyager", "yugabyte"]):
            return AIFrameworkResponse(
                success=True,
                message="Supervisor providing general migration guidance",
                agent_response="Hello! I'm the supervisor agent for YB Voyager migrations. I can help you with:\n\n• Configuration files (create, validate, modify)\n• Schema operations (coming soon)\n• Data migration (coming soon)\n• Validation (coming soon)\n\nWhat would you like to work on?",
                agent_type="supervisor",
                metadata={"available_services": ["configuration", "schema", "data", "validation"]},
                timestamp=datetime.utcnow().isoformat()
            )
        
        # Default response
        else:
            return AIFrameworkResponse(
                success=True,
                message="Supervisor general response",
                agent_response="Hello! I'm the YB Voyager migration supervisor. I can help you with database migrations including configuration, schema operations, data migration, and validation. What would you like to work on?",
                agent_type="supervisor",
                metadata={"available_services": ["configuration", "schema", "data", "validation"]},
                timestamp=datetime.utcnow().isoformat()
            )
    
    def _handle_config_agent(self, request: AIFrameworkRequest, settings: Dict[str, Any]) -> AIFrameworkResponse:
        """Handle config agent specific requests"""
        message_lower = request.message.lower()
        
        if "create" in message_lower or "new" in message_lower:
            return AIFrameworkResponse(
                success=True,
                message="Config agent creating configuration",
                agent_response="I'll help you create a new configuration file. What type of migration are you planning? (offline-migration, live-migration, bulk-data-load, etc.)",
                agent_type="config",
                metadata={"action": "create_config", "available_templates": ["offline-migration", "live-migration", "bulk-data-load"]},
                timestamp=datetime.utcnow().isoformat()
            )
        elif "validate" in message_lower or "check" in message_lower:
            return AIFrameworkResponse(
                success=True,
                message="Config agent validating configuration",
                agent_response="I'll help you validate your configuration file. Please provide the path to your config file or paste its contents.",
                agent_type="config",
                metadata={"action": "validate_config"},
                timestamp=datetime.utcnow().isoformat()
            )
        elif "modify" in message_lower or "edit" in message_lower:
            return AIFrameworkResponse(
                success=True,
                message="Config agent modifying configuration",
                agent_response="I'll help you modify your configuration file. What changes would you like to make?",
                agent_type="config",
                metadata={"action": "modify_config"},
                timestamp=datetime.utcnow().isoformat()
            )
        else:
            return AIFrameworkResponse(
                success=True,
                message="Config agent general help",
                agent_response="I'm the config agent! I can help you create, validate, and modify YB Voyager configuration files. What would you like to do?",
                agent_type="config",
                metadata={"available_actions": ["create", "validate", "modify", "help"]},
                timestamp=datetime.utcnow().isoformat()
            )

# Global AI Framework instance
ai_framework = MockAIFramework()

# Endpoints
@router.get("/status")
async def get_agent_status():
    """Get the status of all AI agents"""
    try:
        return ai_framework.get_agent_status()
    except Exception as e:
        logger.error(f"Error getting agent status: {e}")
        raise HTTPException(status_code=500, detail=f"Failed to get agent status: {str(e)}")

@router.post("/chat")
async def process_ai_chat(
    request: AIFrameworkRequest,
    db = Depends(get_db)
):
    """Process a chat message through the AI framework (always goes to supervisor)"""
    try:
        # Always route through supervisor - let it handle internal routing
        supervisor_request = AIFrameworkRequest(
            message=request.message,
            session_id=request.session_id,
            migration_id=request.migration_id,
            agent_type="supervisor",  # Force supervisor routing
            context=request.context
        )
        
        # Save user message to database
        user_message = Message(
            session_id=request.session_id,
            migration_id=request.migration_id,
            type=MessageType.USER,
            content=request.message
        )
        db.add(user_message)
        db.commit()
        
        # Process through AI framework (supervisor will handle routing)
        response = ai_framework.process_message(supervisor_request)
        
        # Save AI response to database
        ai_message = Message(
            session_id=request.session_id,
            migration_id=request.migration_id,
            type=MessageType.AGENT,
            content=response.agent_response
        )
        db.add(ai_message)
        db.commit()
        
        return response
        
    except Exception as e:
        logger.error(f"Error processing AI chat: {e}")
        raise HTTPException(status_code=500, detail=f"Failed to process chat: {str(e)}")

@router.get("/settings")
async def get_ai_framework_settings():
    """Get current AI framework settings"""
    try:
        settings = load_settings()
        return {
            "model": settings.get("model"),
            "provider": "Google Vertex AI" if settings.get("model", "").startswith("gemini") else "Other",
            "has_credentials": bool(settings.get("google_application_credentials") or settings.get("openai_key") or settings.get("anthropic_key")),
            "framework_initialized": bool(ai_framework.settings)
        }
    except Exception as e:
        logger.error(f"Error getting AI framework settings: {e}")
        raise HTTPException(status_code=500, detail=f"Failed to get settings: {str(e)}")

@router.post("/initialize")
async def initialize_ai_framework():
    """Initialize the AI framework with current settings"""
    try:
        settings = load_settings()
        success = ai_framework.initialize_with_settings(settings)
        return {
            "success": success,
            "message": "AI Framework initialized successfully" if success else "Failed to initialize AI Framework",
            "settings_used": {
                "model": settings.get("model"),
                "provider": "Google Vertex AI" if settings.get("model", "").startswith("gemini") else "Other"
            }
        }
    except Exception as e:
        logger.error(f"Error initializing AI framework: {e}")
        raise HTTPException(status_code=500, detail=f"Failed to initialize AI framework: {str(e)}") 
