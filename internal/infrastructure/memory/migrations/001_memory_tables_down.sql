-- Rollback: 001_memory_tables_down.sql
-- Description: Rollback memory tables migration
-- Author: Self-Organizing Memory v2
-- Date: 2026-02-15

-- Drop triggers first
DROP TRIGGER IF EXISTS memory_cells_au;
DROP TRIGGER IF EXISTS memory_cells_ad;
DROP TRIGGER IF EXISTS memory_cells_ai;

-- Drop FTS virtual table
DROP TABLE IF EXISTS memory_cells_fts;

-- Drop regular tables
DROP TABLE IF EXISTS memory_scenes;
DROP TABLE IF EXISTS memory_cells;
