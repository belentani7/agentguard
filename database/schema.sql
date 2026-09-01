-- AgentGuard SQLite Schema
-- The sentinel's memory

-- Individual cost entries parsed from AI tool logs
CREATE TABLE IF NOT EXISTS cost_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    tool TEXT NOT NULL,
    model TEXT DEFAULT '',
    tokens_in INTEGER DEFAULT 0,
    tokens_out INTEGER DEFAULT 0,
    cost REAL DEFAULT 0.0,
    session_id TEXT DEFAULT '',
    raw_line TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_cost_timestamp ON cost_entries(timestamp);
CREATE INDEX IF NOT EXISTS idx_cost_tool ON cost_entries(tool);
CREATE INDEX IF NOT EXISTS idx_cost_date ON cost_entries(DATE(timestamp));

-- Budget limits
CREATE TABLE IF NOT EXISTS budgets (
    period TEXT PRIMARY KEY,       -- 'daily' or 'weekly'
    amount REAL NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tracked agent processes
CREATE TABLE IF NOT EXISTS agent_processes (
    pid INTEGER PRIMARY KEY,
    tool TEXT NOT NULL,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    paused INTEGER DEFAULT 0       -- 0 = running, 1 = paused (SIGSTOP)
);

-- Smart routing decisions
CREATE TABLE IF NOT EXISTS routing_decisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    from_model TEXT,
    to_model TEXT,
    reason TEXT,
    estimated_savings REAL DEFAULT 0.0
);

-- Activity log
CREATE TABLE IF NOT EXISTS log_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    type TEXT NOT NULL,            -- 'cost', 'alert', 'pause', 'resume', 'route'
    tool TEXT DEFAULT '',
    message TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_log_timestamp ON log_events(timestamp);

-- File watcher state (resumable log scanning)
CREATE TABLE IF NOT EXISTS file_positions (
    file_path TEXT PRIMARY KEY,
    byte_offset INTEGER DEFAULT 0
);

-- Default budgets
INSERT OR IGNORE INTO budgets (period, amount) VALUES ('daily', 10.00);
INSERT OR IGNORE INTO budgets (period, amount) VALUES ('weekly', 50.00);
