from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session
from typing import List

from core.database import get_db
from models.migration import Migration
from api.schemas.migration import MigrationListResponse

router = APIRouter()

@router.get("/", response_model=List[MigrationListResponse])
async def list_projects(
    skip: int = 0, 
    limit: int = 100, 
    db: Session = Depends(get_db)
):
    """Get all migration projects (alias for migrations)"""
    migrations = db.query(Migration).offset(skip).limit(limit).all()
    return migrations

@router.get("/stats")
async def get_project_stats(db: Session = Depends(get_db)):
    """Get project statistics"""
    total_projects = db.query(Migration).count()
    running_projects = db.query(Migration).filter(Migration.status == "running").count()
    completed_projects = db.query(Migration).filter(Migration.status == "completed").count()
    failed_projects = db.query(Migration).filter(Migration.status == "failed").count()
    
    return {
        "total_projects": total_projects,
        "running_projects": running_projects,
        "completed_projects": completed_projects,
        "failed_projects": failed_projects,
        "idle_projects": total_projects - running_projects - completed_projects - failed_projects
    } 
