from pydantic_settings import BaseSettings
from typing import Optional, List
import os

class Settings(BaseSettings):
    # App settings
    app_name: str = "YB Voyager Migration Agent"
    app_version: str = "1.0.0"
    debug: bool = True
    
    # Database settings
    database_url: str = "sqlite:///./migration_agent.db"
    
    # Redis settings
    redis_url: str = "redis://localhost:6379"
    
    # CORS settings
    allowed_origins: List[str] = [
        "http://localhost:3000",
        "http://10.9.117.96:3000"
    ]
    
    # MCP Server settings
    mcp_server_host: str = "localhost"
    mcp_server_port: int = 8001
    
    # Agent settings
    agent_timeout: int = 300  # 5 minutes
    max_concurrent_migrations: int = 3
    
    # File storage
    upload_dir: str = "./uploads"
    temp_dir: str = "./temp"
    
    # Logging
    log_level: str = "INFO"
    
    class Config:
        env_file = ".env"
        case_sensitive = False

# Create settings instance
settings = Settings()

# Ensure directories exist
os.makedirs(settings.upload_dir, exist_ok=True)
os.makedirs(settings.temp_dir, exist_ok=True) 
