package daemon

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite connection.
type DB struct {
	*sql.DB
}

// OpenDB opens the SQLite database and ensures the schema exists.
func OpenDB(path string) *DB {
	os.MkdirAll(filepath.Dir(path), 0755)
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot open database: %v\n", err)
		os.Exit(1)
	}

	// Ensure tables exist
	db.Exec(`
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

		CREATE TABLE IF NOT EXISTS budgets (
			period TEXT PRIMARY KEY,
			amount REAL NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS agent_processes (
			pid INTEGER PRIMARY KEY,
			tool TEXT NOT NULL,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			paused INTEGER DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS routing_decisions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			from_model TEXT,
			to_model TEXT,
			reason TEXT,
			estimated_savings REAL DEFAULT 0.0
		);

		CREATE TABLE IF NOT EXISTS log_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			type TEXT NOT NULL,
			tool TEXT DEFAULT '',
			message TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS file_positions (
			file_path TEXT PRIMARY KEY,
			byte_offset INTEGER DEFAULT 0
		);
	`)

	return &DB{db}
}

// Config holds the daemon configuration.
type Config struct {
	Budget struct {
		Daily  float64 `yaml:"daily"`
		Weekly float64 `yaml:"weekly"`
	} `yaml:"budget"`
	Alerts struct {
		Thresholds   []int  `yaml:"thresholds"`
		Email        string `yaml:"email"`
		SlackWebhook string `yaml:"slack_webhook"`
	} `yaml:"alerts"`
	Routing struct {
		FallbackModel     string  `yaml:"fallback_model"`
		QualityThreshold  float64 `yaml:"quality_threshold"`
	} `yaml:"routing"`
	MonitoredTools []MonitoredTool `yaml:"monitored_tools"`
}

// MonitoredTool defines a tool to watch.
type MonitoredTool struct {
	Name       string `yaml:"name"`
	LogPattern string `yaml:"log_pattern"`
	CostRegex  string `yaml:"cost_regex"`
}

// LoadConfig loads configuration from a YAML file.
func LoadConfig(path string) Config {
	cfg := Config{}

	// Defaults
	cfg.Budget.Daily = 10.00
	cfg.Budget.Weekly = 50.00
	cfg.Alerts.Thresholds = []int{80, 95, 100}
	cfg.Routing.FallbackModel = "qwen"
	cfg.Routing.QualityThreshold = 0.8

	// Try to read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	// Simple YAML parsing (minimal, avoids external dependency for now)
	parseYAML(string(data), &cfg)
	return cfg
}

// parseYAML does a simple line-by-line YAML parse for our config structure.
func parseYAML(data string, cfg *Config) {
	lines := strings.Split(data, "\n")
	var section, subsection string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))

		if indent == 0 && strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			subsection = ""
			continue
		}

		if indent == 2 && strings.HasSuffix(trimmed, ":") {
			subsection = strings.TrimSuffix(trimmed, ":")
			continue
		}

		if strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"'")

			switch section {
			case "budget":
				switch key {
				case "daily":
					fmt.Sscanf(val, "%f", &cfg.Budget.Daily)
				case "weekly":
					fmt.Sscanf(val, "%f", &cfg.Budget.Weekly)
				}
			case "alerts":
				switch key {
				case "email":
					cfg.Alerts.Email = val
				case "slack_webhook":
					cfg.Alerts.SlackWebhook = val
				}
			case "routing":
				switch key {
				case "fallback_model":
					cfg.Routing.FallbackModel = val
				case "quality_threshold":
					fmt.Sscanf(val, "%f", &cfg.Routing.QualityThreshold)
				}
			}
			_ = subsection
		}
	}
}

// CostSnapshot represents aggregated costs at a point in time.
type CostSnapshot struct {
	Tool         string
	Model        string
	TokensIn     int64
	TokensOut    int64
	Cost         float64
	Timestamp    time.Time
}

// Monitor watches log files and extracts cost data.
type Monitor struct {
	db        *DB
	cfg       Config
	mu        sync.Mutex
	stopCh    chan struct{}
	fileState map[string]int64 // file -> byte offset
}

// NewMonitor creates a new monitor.
func NewMonitor(db *DB, cfg Config) *Monitor {
	m := &Monitor{
		db:        db,
		cfg:       cfg,
		stopCh:    make(chan struct{}),
		fileState: make(map[string]int64),
	}

	// Load saved file positions
	rows, err := db.Query("SELECT file_path, byte_offset FROM file_positions")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var path string
			var offset int64
			rows.Scan(&path, &offset)
			m.fileState[path] = offset
		}
	}

	return m
}

// Scan checks all monitored log files for new cost data.
func (m *Monitor) Scan() []CostSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	var snapshots []CostSnapshot

	for _, tool := range m.cfg.MonitoredTools {
		pattern := expandPath(tool.LogPattern)
		files, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, file := range files {
			costs := m.scanFile(file, tool)
			snapshots = append(snapshots, costs...)
		}
	}

	// Also scan known AI tool log directories
	m.scanKnownLocations(&snapshots)

	return snapshots
}

// scanFile reads new content from a log file and extracts cost entries.
func (m *Monitor) scanFile(path string, tool MonitoredTool) []CostSnapshot {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	offset := m.fileState[path]
	if _, err := f.Seek(offset, 0); err != nil {
		return nil
	}

	costRe, err := regexp.Compile(tool.CostRegex)
	if err != nil {
		return nil
	}

	var snapshots []CostSnapshot
	scanner := bufio.NewScanner(f)
	bytesRead := offset

	for scanner.Scan() {
		line := scanner.Text()
		bytesRead += int64(len(line)) + 1 // +1 for newline

		matches := costRe.FindStringSubmatch(line)
		if len(matches) >= 2 {
			var cost float64
			fmt.Sscanf(matches[1], "%f", &cost)

			if cost > 0 {
				snap := CostSnapshot{
					Tool:      tool.Name,
					Cost:      cost,
					Timestamp: time.Now(),
				}

				// Try to extract model name
				if modelMatch := extractModel(line); modelMatch != "" {
					snap.Model = modelMatch
				}

				// Store in database
				m.db.Exec(
					"INSERT INTO cost_entries (tool, model, cost, raw_line, timestamp) VALUES (?, ?, ?, ?, ?)",
					snap.Tool, snap.Model, snap.Cost, line, snap.Timestamp,
				)

				// Log event
				m.db.Exec(
					"INSERT INTO log_events (type, tool, message) VALUES (?, ?, ?)",
					"cost", snap.Tool, fmt.Sprintf("$%.4f (%s)", snap.Cost, snap.Model),
				)

				snapshots = append(snapshots, snap)
			}
		}
	}

	// Save file position
	m.fileState[path] = bytesRead
	m.db.Exec(
		"INSERT OR REPLACE INTO file_positions (file_path, byte_offset) VALUES (?, ?)",
		path, bytesRead,
	)

	return snapshots
}

// scanKnownLocations checks well-known AI tool log directories.
func (m *Monitor) scanKnownLocations(snapshots *[]CostSnapshot) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	knownTools := []struct {
		name    string
		dir     string
		pattern string
	}{
		{"claude", filepath.Join(home, ".claude", "logs"), "*.log"},
		{"aider", filepath.Join(home, ".aider", "logs"), "*.log"},
		{"codex", filepath.Join(home, ".codex", "logs"), "*.log"},
		{"qwen", filepath.Join(home, ".qwen", "logs"), "*.log"},
	}

	defaultCostRe := regexp.MustCompile(`(?i)(?:cost|total|spent|price)["\s:=]+\$?([\d.]+)`)

	for _, kt := range knownTools {
		// Check if already in config
		alreadyConfigured := false
		for _, mt := range m.cfg.MonitoredTools {
			if mt.Name == kt.name {
				alreadyConfigured = true
				break
			}
		}
		if alreadyConfigured {
			continue
		}

		files, err := filepath.Glob(filepath.Join(kt.dir, kt.pattern))
		if err != nil {
			continue
		}

		for _, file := range files {
			f, err := os.Open(file)
			if err != nil {
				continue
			}

			offset := m.fileState[file]
			f.Seek(offset, 0)

			scanner := bufio.NewScanner(f)
			bytesRead := offset

			for scanner.Scan() {
				line := scanner.Text()
				bytesRead += int64(len(line)) + 1

				matches := defaultCostRe.FindStringSubmatch(line)
				if len(matches) >= 2 {
					var cost float64
					fmt.Sscanf(matches[1], "%f", &cost)
					if cost > 0 {
						snap := CostSnapshot{
							Tool:      kt.name,
							Cost:      cost,
							Timestamp: time.Now(),
						}
						*snapshots = append(*snapshots, snap)

						m.db.Exec(
							"INSERT INTO cost_entries (tool, cost, raw_line, timestamp) VALUES (?, ?, ?, ?)",
							kt.name, cost, line, time.Now(),
						)
					}
				}
			}

			f.Close()
			m.fileState[file] = bytesRead
			m.db.Exec(
				"INSERT OR REPLACE INTO file_positions (file_path, byte_offset) VALUES (?, ?)",
				file, bytesRead,
			)
		}
	}
}

// Shutdown gracefully stops the monitor.
func (m *Monitor) Shutdown() {
	close(m.stopCh)
	m.mu.Lock()
	defer m.mu.Unlock()

	// Persist file positions
	for path, offset := range m.fileState {
		m.db.Exec(
			"INSERT OR REPLACE INTO file_positions (file_path, byte_offset) VALUES (?, ?)",
			path, offset,
		)
	}
}

// SetBudget updates a budget limit in the database.
func SetBudget(db *DB, period string, amount float64) {
	db.Exec(
		"INSERT OR REPLACE INTO budgets (period, amount, updated_at) VALUES (?, ?, ?)",
		period, amount, time.Now(),
	)
}

// LogEntry represents a log event.
type LogEntry struct {
	Timestamp time.Time
	Type      string
	Tool      string
	Message   string
}

// RecentLogs returns the most recent log entries.
func RecentLogs(db *DB, limit int) []LogEntry {
	rows, err := db.Query(
		"SELECT timestamp, type, tool, message FROM log_events ORDER BY timestamp DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var e LogEntry
		var ts string
		rows.Scan(&ts, &e.Type, &e.Tool, &e.Message)
		e.Timestamp, _ = time.Parse("2006-01-02 15:04:05", ts)
		entries = append(entries, e)
	}

	// Reverse for chronological order
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries
}

// expandPath expands ~ to the home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// extractModel tries to find a model name in a log line.
func extractModel(line string) string {
	modelRe := regexp.MustCompile(`(?i)(?:model|engine)["\s:=]+["']?([\w-]+(?:\.[\w-]+)*)`)
	matches := modelRe.FindStringSubmatch(line)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}
