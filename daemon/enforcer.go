package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/belentani7/agentguard/notifications"
)

// Enforcer checks spending against budgets and takes action.
type Enforcer struct {
	db             *DB
	cfg            Config
	mu             sync.Mutex
	stopCh         chan struct{}
	alertSent      map[string]bool // threshold key -> sent
	pausedPIDs     map[int]bool
	lastDailyReset time.Time
	lastWeekReset  time.Time
}

// NewEnforcer creates a new budget enforcer.
func NewEnforcer(db *DB, cfg Config) *Enforcer {
	return &Enforcer{
		db:             db,
		cfg:            cfg,
		stopCh:         make(chan struct{}),
		alertSent:      make(map[string]bool),
		pausedPIDs:     make(map[int]bool),
		lastDailyReset: startOfDay(time.Now()),
		lastWeekReset:  startOfWeek(time.Now()),
	}
}

// Check evaluates current spending and enforces limits.
func (e *Enforcer) Check(snapshots []CostSnapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Reset alert state on new day/week
	now := time.Now()
	if now.After(e.lastDailyReset.Add(24 * time.Hour)) {
		e.lastDailyReset = startOfDay(now)
		e.resetAlerts("daily")
		e.resumeIfBudgetReset("daily")
	}
	if now.After(e.lastWeekReset.Add(7 * 24 * time.Hour)) {
		e.lastWeekReset = startOfWeek(now)
		e.resetAlerts("weekly")
		e.resumeIfBudgetReset("weekly")
	}

	// Get daily spend
	dailySpend := e.getSpendSince(e.lastDailyReset)
	weeklySpend := e.getSpendSince(e.lastWeekReset)

	// Get budget limits
	dailyBudget := e.getBudget("daily")
	weeklyBudget := e.getBudget("weekly")

	// Check daily limit
	if dailyBudget > 0 {
		pct := (dailySpend / dailyBudget) * 100
		e.checkThresholds("daily", pct, dailySpend, dailyBudget)

		if pct >= 100 {
			e.pauseAllAgents()
		}
	}

	// Check weekly limit
	if weeklyBudget > 0 {
		pct := (weeklySpend / weeklyBudget) * 100
		e.checkThresholds("weekly", pct, weeklySpend, weeklyBudget)

		if pct >= 100 {
			e.pauseAllAgents()
		}
	}
}

// checkThresholds sends alerts at configured thresholds.
func (e *Enforcer) checkThresholds(period string, pct float64, spent, budget float64) {
	for _, threshold := range e.cfg.Alerts.Thresholds {
		key := fmt.Sprintf("%s_%d", period, threshold)
		if pct >= float64(threshold) && !e.alertSent[key] {
			e.alertSent[key] = true

			level := "warning"
			if threshold >= 100 {
				level = "critical"
			} else if threshold >= 95 {
				level = "high"
			}

			title := fmt.Sprintf("🔒 %s Budget Alert (%d%%)", strings.Title(period), threshold)
			msg := fmt.Sprintf("$%.2f / $%.2f spent", spent, budget)

			if threshold >= 100 {
				msg += " — AGENTS PAUSED"
			}

			notifications.Send(notifications.Alert{
				Level:   level,
				Title:   title,
				Message: msg,
			})

			// Log event
			e.db.Exec(
				"INSERT INTO log_events (type, tool, message) VALUES (?, ?, ?)",
				"alert", period, fmt.Sprintf("%d%% threshold: $%.2f/$%.2f", threshold, spent, budget),
			)
		}
	}
}

// pauseAllAgents sends SIGSTOP to all known agent processes.
func (e *Enforcer) pauseAllAgents() {
	// Find running AI tool processes
	pids := findAgentProcesses()

	for _, pid := range pids {
		if e.pausedPIDs[pid] {
			continue
		}

		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}

		if err := sendStopSignal(proc); err == nil {
			e.pausedPIDs[pid] = true
			e.db.Exec(
				"INSERT OR REPLACE INTO agent_processes (pid, tool, paused) VALUES (?, ?, 1)",
				pid, "unknown",
			)
			e.db.Exec(
				"INSERT INTO log_events (type, tool, message) VALUES (?, ?, ?)",
				"pause", "system", fmt.Sprintf("Paused agent PID %d", pid),
			)
		}
	}
}

// resumeAllAgents resumes all paused agent processes.
func (e *Enforcer) resumeAllAgents() {
	for pid := range e.pausedPIDs {
		proc, err := os.FindProcess(pid)
		if err != nil {
			delete(e.pausedPIDs, pid)
			continue
		}

		if err := sendContSignal(proc); err == nil {
			delete(e.pausedPIDs, pid)
			e.db.Exec(
				"UPDATE agent_processes SET paused = 0 WHERE pid = ?",
				pid,
			)
			e.db.Exec(
				"INSERT INTO log_events (type, tool, message) VALUES (?, ?, ?)",
				"resume", "system", fmt.Sprintf("Resumed agent PID %d", pid),
			)
		}
	}
}

// resumeIfBudgetReset resumes agents when a budget period resets.
func (e *Enforcer) resumeIfBudgetReset(period string) {
	e.resumeAllAgents()
	e.db.Exec(
		"INSERT INTO log_events (type, tool, message) VALUES (?, ?, ?)",
		"resume", period, fmt.Sprintf("%s budget reset — agents resumed", period),
	)
}

// resetAlerts clears alert state for a given period.
func (e *Enforcer) resetAlerts(period string) {
	for key := range e.alertSent {
		if strings.HasPrefix(key, period+"_") {
			delete(e.alertSent, key)
		}
	}
}

// getSpendSince returns total spend since a given time.
func (e *Enforcer) getSpendSince(since time.Time) float64 {
	var total float64
	row := e.db.QueryRow(
		"SELECT COALESCE(SUM(cost), 0) FROM cost_entries WHERE timestamp >= ?",
		since.Format("2006-01-02 15:04:05"),
	)
	row.Scan(&total)
	return total
}

// getBudget retrieves the budget amount for a period.
func (e *Enforcer) getBudget(period string) float64 {
	var amount float64
	row := e.db.QueryRow("SELECT amount FROM budgets WHERE period = ?", period)
	err := row.Scan(&amount)
	if err != nil {
		// Fall back to config defaults
		switch period {
		case "daily":
			return e.cfg.Budget.Daily
		case "weekly":
			return e.cfg.Budget.Weekly
		}
		return 0
	}
	return amount
}

// Shutdown gracefully stops the enforcer.
func (e *Enforcer) Shutdown() {
	// Resume all paused agents on shutdown
	e.resumeAllAgents()
	close(e.stopCh)
}

// PauseAllAgents pauses all known agent processes (manual command).
func PauseAllAgents(db *DB) int {
	pids := findAgentProcesses()
	count := 0

	for _, pid := range pids {
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		if err := sendStopSignal(proc); err == nil {
			count++
			db.Exec(
				"INSERT OR REPLACE INTO agent_processes (pid, tool, paused) VALUES (?, ?, 1)",
				pid, "unknown",
			)
			db.Exec(
				"INSERT INTO log_events (type, tool, message) VALUES (?, ?, ?)",
				"pause", "system", fmt.Sprintf("Manual pause: PID %d", pid),
			)
		}
	}
	return count
}

// ResumeAllAgents resumes all paused agent processes (manual command).
func ResumeAllAgents(db *DB) int {
	rows, err := db.Query("SELECT pid FROM agent_processes WHERE paused = 1")
	if err != nil {
		return 0
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var pid int
		rows.Scan(&pid)
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		if err := sendContSignal(proc); err == nil {
			count++
			db.Exec("UPDATE agent_processes SET paused = 0 WHERE pid = ?", pid)
			db.Exec(
				"INSERT INTO log_events (type, tool, message) VALUES (?, ?, ?)",
				"resume", "system", fmt.Sprintf("Manual resume: PID %d", pid),
			)
		}
	}
	return count
}

// Forecast holds predicted spend data.
type Forecast struct {
	DaysTracked      int
	AvgDailySpend    float64
	PredictedMonthly float64
	PeakDaySpend     float64
	PeakDayDate      string
	LowDaySpend      float64
	LowDayDate       string
	MonthlyBudget    float64
}

// PredictMonthlySpend forecasts monthly spending based on historical data.
func PredictMonthlySpend(db *DB) Forecast {
	f := Forecast{}

	// Count days with data
	db.QueryRow("SELECT COUNT(DISTINCT DATE(timestamp)) FROM cost_entries").Scan(&f.DaysTracked)

	if f.DaysTracked == 0 {
		return f
	}

	// Average daily spend
	db.QueryRow("SELECT COALESCE(SUM(cost), 0) / COUNT(DISTINCT DATE(timestamp)) FROM cost_entries").Scan(&f.AvgDailySpend)

	// Predict monthly (30 days)
	f.PredictedMonthly = f.AvgDailySpend * 30

	// Peak day
	var peakDate string
	db.QueryRow("SELECT DATE(timestamp), SUM(cost) as total FROM cost_entries GROUP BY DATE(timestamp) ORDER BY total DESC LIMIT 1").Scan(&peakDate, &f.PeakDaySpend)
	f.PeakDayDate = peakDate

	// Lowest day
	var lowDate string
	db.QueryRow("SELECT DATE(timestamp), SUM(cost) as total FROM cost_entries GROUP BY DATE(timestamp) ORDER BY total ASC LIMIT 1").Scan(&lowDate, &f.LowDaySpend)
	f.LowDayDate = lowDate

	// Monthly budget (weekly * 4.33)
	var weeklyBudget float64
	db.QueryRow("SELECT amount FROM budgets WHERE period = 'weekly'").Scan(&weeklyBudget)
	if weeklyBudget > 0 {
		f.MonthlyBudget = weeklyBudget * 4.33
	} else {
		// Default: daily * 30
		var dailyBudget float64
		db.QueryRow("SELECT amount FROM budgets WHERE period = 'daily'").Scan(&dailyBudget)
		if dailyBudget > 0 {
			f.MonthlyBudget = dailyBudget * 30
		} else {
			f.MonthlyBudget = 300.00 // default
		}
	}

	return f
}

// findAgentProcesses looks for running AI tool processes.
func findAgentProcesses() []int {
	var pids []int

	// Read /proc on Linux to find matching processes
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return pids
	}

	agentNames := []string{"claude", "aider", "codex", "qwen", "opencode", "cursor"}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			continue
		}

		cmdStr := strings.ToLower(string(cmdline))
		for _, name := range agentNames {
			if strings.Contains(cmdStr, name) {
				pids = append(pids, pid)
				break
			}
		}
	}

	return pids
}

// startOfDay returns the start of the given day.
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// startOfWeek returns the start of the week (Monday).
func startOfWeek(t time.Time) time.Time {
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysSinceMonday := int(weekday) - 1
	start := t.AddDate(0, 0, -daysSinceMonday)
	return startOfDay(start)
}
