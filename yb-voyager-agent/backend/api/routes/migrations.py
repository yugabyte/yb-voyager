from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session
from typing import List, Optional
from datetime import datetime

from core.database import get_db
from models.migration import Migration, MigrationStatus, MigrationPhase
from api.schemas.migration import (
    MigrationCreate, 
    MigrationUpdate, 
    MigrationResponse,
    MigrationListResponse
)

router = APIRouter()

@router.get("/", response_model=List[MigrationListResponse])
async def list_migrations(
    skip: int = 0, 
    limit: int = 100, 
    db: Session = Depends(get_db)
):
    """Get all migrations with pagination"""
    migrations = db.query(Migration).offset(skip).limit(limit).all()
    return migrations

@router.post("/", response_model=MigrationResponse, status_code=status.HTTP_201_CREATED)
async def create_migration(
    migration: MigrationCreate, 
    db: Session = Depends(get_db)
):
    """Create a new migration project"""
    
    # Handle config content based on config option
    config_content = migration.config
    if migration.config_content:
        config_content = migration.config_content
    
    # If AI-guided, we'll need to trigger the AI agent to create the config
    if migration.config_option == 'ai_guided':
        # TODO: Trigger AI agent to create config
        # For now, we'll create the migration and let the AI handle config creation later
        pass
    
    db_migration = Migration(
        name=migration.name,
        description=migration.description,
        source_type=migration.source_type,
        source_host=migration.source_host,
        source_port=migration.source_port,
        source_database=migration.source_database,
        target_host=migration.target_host,
        target_port=migration.target_port,
        target_database=migration.target_database,
        config=config_content,
        config_option=migration.config_option,
        config_file_path=migration.config_file_path
    )
    db.add(db_migration)
    db.commit()
    db.refresh(db_migration)
    return db_migration

@router.get("/{migration_id}", response_model=MigrationResponse)
async def get_migration(migration_id: int, db: Session = Depends(get_db)):
    """Get a specific migration by ID"""
    migration = db.query(Migration).filter(Migration.id == migration_id).first()
    if not migration:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND, 
            detail="Migration not found"
        )
    return migration

@router.put("/{migration_id}", response_model=MigrationResponse)
async def update_migration(
    migration_id: int, 
    migration_update: MigrationUpdate, 
    db: Session = Depends(get_db)
):
    """Update a migration project"""
    db_migration = db.query(Migration).filter(Migration.id == migration_id).first()
    if not db_migration:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND, 
            detail="Migration not found"
        )
    
    # Update fields
    update_data = migration_update.dict(exclude_unset=True)
    for field, value in update_data.items():
        setattr(db_migration, field, value)
    
    db_migration.updated_at = datetime.utcnow()
    db.commit()
    db.refresh(db_migration)
    return db_migration

@router.delete("/{migration_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_migration(migration_id: int, db: Session = Depends(get_db)):
    """Delete a migration project"""
    migration = db.query(Migration).filter(Migration.id == migration_id).first()
    if not migration:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND, 
            detail="Migration not found"
        )
    
    db.delete(migration)
    db.commit()
    return None

@router.post("/{migration_id}/start", response_model=MigrationResponse)
async def start_migration(migration_id: int, db: Session = Depends(get_db)):
    """Start a migration project"""
    migration = db.query(Migration).filter(Migration.id == migration_id).first()
    if not migration:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND, 
            detail="Migration not found"
        )
    
    if migration.status == MigrationStatus.RUNNING:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST, 
            detail="Migration is already running"
        )
    
    migration.status = MigrationStatus.RUNNING
    migration.started_at = datetime.utcnow()
    migration.progress_percentage = 0
    db.commit()
    db.refresh(migration)
    return migration

@router.post("/{migration_id}/stop", response_model=MigrationResponse)
async def stop_migration(migration_id: int, db: Session = Depends(get_db)):
    """Stop a running migration"""
    migration = db.query(Migration).filter(Migration.id == migration_id).first()
    if not migration:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND, 
            detail="Migration not found"
        )
    
    if migration.status != MigrationStatus.RUNNING:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST, 
            detail="Migration is not running"
        )
    
    migration.status = MigrationStatus.IDLE
    db.commit()
    db.refresh(migration)
    return migration 
