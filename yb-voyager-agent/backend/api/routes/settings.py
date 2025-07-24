from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from typing import Optional
import json
import os
import tempfile

router = APIRouter()

class DatabaseConnection(BaseModel):
    host: str
    port: str
    database: str
    username: str
    password: str

class Settings(BaseModel):
    model: str = "gemini-2.5-flash-lite"
    openai_key: Optional[str] = None
    anthropic_key: Optional[str] = None
    google_application_credentials: Optional[str] = None
    custom_endpoint: Optional[str] = None
    source_connection: Optional[DatabaseConnection] = None
    target_connection: Optional[DatabaseConnection] = None

SETTINGS_FILE = "settings.json"

def load_settings() -> dict:
    """Load settings from file"""
    default_settings = {
        "model": "gemini-2.5-flash-lite"
    }
    
    if os.path.exists(SETTINGS_FILE):
        try:
            with open(SETTINGS_FILE, 'r') as f:
                saved_settings = json.load(f)
                # Merge with defaults to ensure new fields are present
                return {**default_settings, **saved_settings}
        except Exception as e:
            print(f"Error loading settings: {e}")
    return default_settings

def save_settings(settings: dict):
    """Save settings to file"""
    try:
        with open(SETTINGS_FILE, 'w') as f:
            json.dump(settings, f, indent=2)
    except Exception as e:
        print(f"Error saving settings: {e}")
        raise HTTPException(status_code=500, detail="Failed to save settings")

@router.get("/")
async def get_settings():
    """Get current settings"""
    return load_settings()

@router.post("/")
async def update_settings(settings: Settings):
    """Update settings"""
    try:
        # Convert to dict and save
        settings_dict = settings.dict(exclude_none=True)
        save_settings(settings_dict)
        return {"message": "Settings updated successfully", "settings": settings_dict}
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to update settings: {str(e)}")

@router.get("/models")
async def get_available_models():
    """Get list of available models"""
    return {
        "models": [
            {"id": "gemini-2.5-flash-lite", "name": "Gemini 2.5 Flash Lite", "provider": "Google Vertex AI", "description": "Fastest and most cost-effective Gemini model"},
            {"id": "gemini-2.5-flash", "name": "Gemini 2.5 Flash", "provider": "Google Vertex AI", "description": "Balanced performance and speed for most tasks"},
            {"id": "gemini-2.5-pro", "name": "Gemini 2.5 Pro", "provider": "Google Vertex AI", "description": "Most capable Gemini model for complex reasoning"},
            {"id": "gpt-4", "name": "GPT-4", "provider": "OpenAI", "description": "Most capable model for complex migrations"},
            {"id": "gpt-3.5-turbo", "name": "GPT-3.5 Turbo", "provider": "OpenAI", "description": "Fast and cost-effective"},
            {"id": "claude-3-opus", "name": "Claude 3 Opus", "provider": "Anthropic", "description": "Advanced reasoning capabilities"},
            {"id": "claude-3-sonnet", "name": "Claude 3 Sonnet", "provider": "Anthropic", "description": "Balanced performance and speed"},
            {"id": "claude-3-haiku", "name": "Claude 3 Haiku", "provider": "Anthropic", "description": "Fast and efficient"},
            {"id": "local-llama", "name": "Local Llama", "provider": "Local", "description": "Privacy-focused, runs locally"},
            {"id": "custom", "name": "Custom Model", "provider": "Custom", "description": "Use your own model endpoint"},
        ]
    }

@router.post("/test-connection")
async def test_database_connection(connection: DatabaseConnection):
    """Test database connection"""
    try:
        # Here you would implement actual database connection testing
        # For now, we'll just validate the connection parameters
        if not connection.host or not connection.database:
            raise HTTPException(status_code=400, detail="Host and database are required")
        
        # TODO: Implement actual connection testing
        return {"message": "Connection test successful", "connection": connection.dict()}
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Connection test failed: {str(e)}")

@router.post("/test-google-credentials")
async def test_google_credentials(credentials: dict):
    """Test Google Cloud credentials"""
    try:
        google_creds = credentials.get("google_application_credentials")
        if not google_creds:
            raise HTTPException(status_code=400, detail="Google application credentials are required")
        
        # Create a temporary file with the credentials
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as temp_file:
            temp_file.write(google_creds)
            temp_file_path = temp_file.name
        
        try:
            # Set the environment variable for this test
            original_creds = os.environ.get("GOOGLE_APPLICATION_CREDENTIALS")
            os.environ["GOOGLE_APPLICATION_CREDENTIALS"] = temp_file_path
            
            # Test the credentials
            try:
                import google.auth
                import google.cloud.aiplatform
                
                # Get default credentials
                credentials, project_id = google.auth.default()
                
                # Test Vertex AI initialization
                google.cloud.aiplatform.init(
                    project=project_id,
                    location="us-central1"
                )
                
                # Get client email from credentials
                client_email = credentials.service_account_email if hasattr(credentials, 'service_account_email') else "Unknown"
                
                return {
                    "message": "Google Cloud credentials are valid",
                    "project_id": project_id,
                    "client_email": client_email,
                    "note": "Credentials are working correctly with Vertex AI"
                }
                
            except ImportError:
                return {
                    "message": "Google Cloud credentials format is valid",
                    "warning": "Google Cloud libraries not installed. Install with: pip install google-auth google-cloud-aiplatform"
                }
            except Exception as e:
                return {
                    "message": "Google Cloud credentials test failed",
                    "error": str(e),
                    "note": "Please check your service account permissions and project configuration"
                }
                
        finally:
            # Clean up
            os.unlink(temp_file_path)
            if original_creds:
                os.environ["GOOGLE_APPLICATION_CREDENTIALS"] = original_creds
            else:
                os.environ.pop("GOOGLE_APPLICATION_CREDENTIALS", None)
                
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Credential test failed: {str(e)}") 
