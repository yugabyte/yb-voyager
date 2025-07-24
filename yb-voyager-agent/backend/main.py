from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles
import logging
from core.database import init_db
from api.routes import migrations, chat, projects
from api.routes import settings
from api.routes import ai_framework

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(
    title="YB Voyager Migration Agent",
    description="AI-powered database migration assistant",
    version="1.0.0"
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # In production, specify your frontend URL
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include routers
app.include_router(migrations.router, prefix="/api/migrations", tags=["migrations"])
app.include_router(chat.router, prefix="/api/chat", tags=["chat"])
app.include_router(projects.router, prefix="/api/projects", tags=["projects"])
app.include_router(settings.router, prefix="/api/settings", tags=["settings"])
app.include_router(ai_framework.router, prefix="/api/ai-framework", tags=["ai-framework"])

@app.on_event("startup")
async def startup_event():
    logger.info("Starting YB Voyager Migration Agent Backend")
    await init_db()

@app.get("/")
async def root():
    return {"message": "YB Voyager Migration Agent API", "version": "1.0.0"}

@app.get("/health")
async def health_check():
    return {"status": "healthy", "service": "yb-voyager-agent"}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000) 
