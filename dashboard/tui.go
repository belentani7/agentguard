package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/belentani7/agentguard/daemon"
)

// Run starts the TUI dashboard.
func Run(db *daemon.DB, cfg daemon.Config) {
	render(db, cfg)
}

// render draws the dashboard.
func render(db *daemon.DB, cfg daemon.Config) {
	clearScreen()

	// Gather data
	dailySpend := getDailySpend(db)
	weeklySpend := getWeeklySpend(db)
	dailyBudget := getBudget(db, "daily", cfg.Budget.Daily)
	weeklyBudget := getBudget(db, "weekly", cfg.Budget.Weekly)
	toolBreakdown := getToolBreakdown(db)
	recentLogs := daemon.RecentLogs(db, 10)
	activeAgents := getActiveAgents(db)

	// Header
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  🔒 AgentGuard — The Sentinel Dashboard")
	fmt.Printf("  %s\n", time.Now().Format("Monday, January 2, 2006 — 15:04:05"))
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Budget bars
	fmt.Println("  ┌─ BUDGET ─────────────────────────────────────────────────────────┐")
	drawBudgetBar("  │ Daily ", dailySpend, dailyBudget, 50)
	drawBudgetBar("  │ Weekly", weeklySpend, weeklyBudget, 50)
	fmt.Println("  └───────────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// Tool breakdown
	fmt.Println("  ┌─ BY TOOL ─────────────────────────────────────────────────────────┐")
	if len(toolBreakdown) == 0 {
		fmt.Println("  │ No spending recorded yet                                        │")
	} else {
		for tool, cost := range toolBreakdown {
			pct := 0.0
			if dailySpend > 0 {
				pct = (cost / dailySpend) * 100
			}
			bar := drawBar(pct, 30)
			fmt.Printf("  │ %-10s $%7.2f [%s] %3.0f%%\n", tool, cost, bar, pct)
		}
	}
	fmt.Println("  └───────────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// Active agents
	fmt.Println("  ┌─ ACTIVE AGENTS ───────────────────────────────────────────────────┐")
	if len(activeAgents) == 0 {
		fmt.Println("  │ No agents detected                                               │")
	} else {
		for _, agent := range activeAgents {
			status := "🟢 running"
			if agent.Paused {
				status = "🔴 paused"
			}
			fmt.Printf("  │ PID %-6d %-12s %s\n", agent.PID, agent.Tool, status)
		}
	}
	fmt.Println("  └───────────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// Recent activity
	fmt.Println("  ┌─ RECENT ACTIVITY ─────────────────────────────────────────────────┐")
	if len(recentLogs) == 0 {
		fmt.Println("  │ No activity yet                                                  │")
	} else {
		for _, e := range recentLogs {
			icon := "📊"
			switch e.Type {
			case "alert":
				icon = "🔔"
			case "pause":
				icon = "⏸️ "
			case "resume":
				icon = "▶️ "
			case "route":
				icon = "🔀"
			}
			fmt.Printf("  │ %s %s %-8s %s\n", icon, e.Timestamp.Format("15:04"), e.Tool, e.Message)
		}
	}
	fmt.Println("  └───────────────────────────────────────────────────────────────────┘")
	fmt.Println()

	// Forecast
	forecast := daemon.PredictMonthlySpend(db)
	if forecast.DaysTracked > 0 {
		fmt.Println("  ┌─ FORECAST ────────────────────────────────────────────────────────┐")
		fmt.Printf("  │ Avg daily: $%.2f  |  Predicted monthly: $%.2f  |  Budget: $%.2f\n",
			forecast.AvgDailySpend, forecast.PredictedMonthly, forecast.MonthlyBudget)
		if forecast.PredictedMonthly > forecast.MonthlyBudget {
			fmt.Println("  │ ⚠️  On track to EXCEED monthly budget!")
		} else {
			fmt.Println("  │ ✅ Within projected budget")
		}
		fmt.Println("  └───────────────────────────────────────────────────────────────────┘")
	}

	fmt.Println()
}

// drawBudgetBar renders a budget progress bar.
func drawBudgetBar(label string, spent, budget float64, width int) {
	pct := 0.0
	if budget > 0 {
		pct = (spent / budget) * 100
	}
	if pct > 100 {
		pct = 100
	}

	filled := int(pct / 100 * float64(width))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	color := "🟢"
	if pct >= 95 {
		color = "🔴"
	} else if pct >= 80 {
		color = "🟡"
	}

	fmt.Printf("%s %s $%7.2f / $%.2f (%3.0f%%)\n", color, bar, spent, budget, pct)
}

// drawBar creates a simple progress bar.
func drawBar(pct float64, width int) string {
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * float64(width))
	return strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)
}

// AgentInfo represents a detected agent process.
type AgentInfo struct {
	PID    int
	Tool   string
	Paused bool
}

func getDailySpend(db *daemon.DB) float64 {
	today := time.Now().Format("2006-01-02")
	var total float64
	db.QueryRow("SELECT COALESCE(SUM(cost), 0) FROM cost_entries WHERE DATE(timestamp) = ?", today).Scan(&total)
	return total
}

func getWeeklySpend(db *daemon.DB) float64 {
	var total float64
	db.QueryRow("SELECT COALESCE(SUM(cost), 0) FROM cost_entries WHERE timestamp >= datetime('now', 'weekday 0', '-7 days')").Scan(&total)
	return total
}

func getBudget(db *daemon.DB, period string, defaultVal float64) float64 {
	var amount float64
	err := db.QueryRow("SELECT amount FROM budgets WHERE period = ?", period).Scan(&amount)
	if err != nil {
		return defaultVal
	}
	return amount
}

func getToolBreakdown(db *daemon.DB) map[string]float64 {
	result := make(map[string]float64)
	today := time.Now().Format("2006-01-02")
	rows, err := db.Query("SELECT tool, SUM(cost) FROM cost_entries WHERE DATE(timestamp) = ? GROUP BY tool", today)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var tool string
		var cost float64
		rows.Scan(&tool, &cost)
		result[tool] = cost
	}
	return result
}

func getActiveAgents(db *daemon.DB) []AgentInfo {
	var agents []AgentInfo
	rows, err := db.Query("SELECT pid, tool, paused FROM agent_processes")
	if err != nil {
		return agents
	}
	defer rows.Close()
	for rows.Next() {
		var a AgentInfo
		rows.Scan(&a.PID, &a.Tool, &a.Paused)
		agents = append(agents, a)
	}
	return agents
}

func clearScreen() {
	fmt.Print("\033[2J\033[H")
}
