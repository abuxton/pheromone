-- Pheromone PostgreSQL initialisation
-- ADR-016: Stateless Server — Dataplane Selection
--
-- This script runs automatically when the postgres container is first
-- created.  It sets up the extensions and placeholder schema that the
-- Pheromone server will use once the PostgreSQL dataplane integration
-- described in ADR-016 is implemented.

-- Extensions -----------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Schema placeholder ---------------------------------------------------------
-- Tables below mirror the conceptual model used by the existing in-memory /
-- etcd hybrid store (ADR-002).  Column types and indexes will be refined
-- during the ADR-016 implementation sprint.

CREATE TABLE IF NOT EXISTS agents (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT        NOT NULL,
    address     TEXT        NOT NULL,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata    JSONB       NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS twins (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id    UUID        REFERENCES agents(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    state       JSONB       NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS changesets (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    twin_id     UUID        NOT NULL REFERENCES twins(id) ON DELETE CASCADE,
    diff        JSONB       NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common query patterns ------------------------------------------
CREATE INDEX IF NOT EXISTS idx_twins_agent_id     ON twins(agent_id);
CREATE INDEX IF NOT EXISTS idx_changesets_twin_id ON changesets(twin_id);
CREATE INDEX IF NOT EXISTS idx_agents_last_seen    ON agents(last_seen_at DESC);
