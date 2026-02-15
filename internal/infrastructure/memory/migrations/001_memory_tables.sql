-- Migration: 001_memory_tables.sql
-- Description: Create memory_cells, memory_scenes, and FTS5 search tables
-- Author: Self-Organizing Memory v2
-- Date: 2026-02-15

-- ============================================================================
-- memory_cells table
-- ============================================================================
-- Stores individual memory cells (typed knowledge units)
CREATE TABLE IF NOT EXISTS memory_cells (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    scene TEXT NOT NULL,
    cell_type TEXT NOT NULL,
    salience REAL NOT NULL,
    content TEXT NOT NULL,
    source TEXT NOT NULL,  -- JSON array of message IDs
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP,

    -- Constraints
    CONSTRAINT chk_salience CHECK (salience >= 0.0 AND salience <= 1.0),
    CONSTRAINT chk_cell_type CHECK (cell_type IN ('fact', 'decision', 'task', 'preference', 'plan', 'risk'))
);

-- Indexes for memory_cells
CREATE INDEX IF NOT EXISTS idx_memory_cells_conversation
    ON memory_cells(conversation_id);

CREATE INDEX IF NOT EXISTS idx_memory_cells_scene
    ON memory_cells(scene);

CREATE INDEX IF NOT EXISTS idx_memory_cells_salience
    ON memory_cells(salience DESC);

CREATE INDEX IF NOT EXISTS idx_memory_cells_expires_at
    ON memory_cells(expires_at)
    WHERE expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_memory_cells_created_at
    ON memory_cells(created_at DESC);

-- Composite index for common query patterns
CREATE INDEX IF NOT EXISTS idx_memory_cells_conv_scene
    ON memory_cells(conversation_id, scene);

-- ============================================================================
-- memory_scenes table
-- ============================================================================
-- Stores consolidated scene summaries
CREATE TABLE IF NOT EXISTS memory_scenes (
    scene TEXT PRIMARY KEY,
    summary TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    -- Constraints
    CONSTRAINT chk_token_count CHECK (token_count > 0 AND token_count <= 2000)
);

-- ============================================================================
-- memory_cells_fts virtual table (FTS5)
-- ============================================================================
-- Full-text search index for fast retrieval
CREATE VIRTUAL TABLE IF NOT EXISTS memory_cells_fts USING fts5(
    content,
    scene,
    cell_type,
    content='memory_cells',
    content_rowid='rowid'
);

-- ============================================================================
-- Triggers to keep FTS table in sync
-- ============================================================================

-- Trigger: Insert new cell into FTS
CREATE TRIGGER IF NOT EXISTS memory_cells_ai
AFTER INSERT ON memory_cells
BEGIN
    INSERT INTO memory_cells_fts(rowid, content, scene, cell_type)
    VALUES (new.rowid, new.content, new.scene, new.cell_type);
END;

-- Trigger: Delete cell from FTS
CREATE TRIGGER IF NOT EXISTS memory_cells_ad
AFTER DELETE ON memory_cells
BEGIN
    DELETE FROM memory_cells_fts WHERE rowid = old.rowid;
END;

-- Trigger: Update cell in FTS (FTS5 requires DELETE + INSERT)
CREATE TRIGGER IF NOT EXISTS memory_cells_au
AFTER UPDATE ON memory_cells
BEGIN
    DELETE FROM memory_cells_fts WHERE rowid = old.rowid;
    INSERT INTO memory_cells_fts(rowid, content, scene, cell_type)
    VALUES (new.rowid, new.content, new.scene, new.cell_type);
END;
