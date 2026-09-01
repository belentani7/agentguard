package daemon

import (
	"fmt"
	"sync"
	"time"

	"github.com/belentani7/agentguard/notifications"
)

// Model cost tiers (approximate cost per 1M tokens).
var modelTiers = map[string]float64{
	"claude-opus":     75.00,
	"claude-sonnet":   15.00,
	"claude-haiku":    1.00,
	"gpt-4o":          10.00,
	"gpt-4o-mini":     0.30,
	"qwen-max":        3.50,
	"qwen-plus":       0.80,
	"qwen-turbo":      0.30,
	"deepseek-chat":   0.27,
	"deepseek-coder":  0.27,
	"gemini-pro":      3.50,
	"gemini-flash":    0.075,
}

// RoutingDecision represents a model routing action.
type RoutingDecision struct {
	FromModel        string
	ToModel          string
	Reason           string
	EstimatedSavings float64
}

// Router handles smart model routing based on budget.
type Router struct {
	db     *DB
	cfg    Config
	mu     sync.Mutex
	stopCh chan struct{}
}

// NewRouter creates a new smart router.
func NewRouter(db *DB, cfg Config) *Router {
	return &Router{
		db:     db,
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

// Evaluate checks if routing changes are needed based on current spend.
func (r *Router) Evaluate(snapshots []CostSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get current daily spend
	dailySpend := r.getTodaySpend()
	dailyBudget := r.getBudget("daily")

	if dailyBudget <= 0 {
		return
	}

	pctUsed := (dailySpend / dailyBudget) * 100

	// If premium budget is getting tight (>70%), suggest routing to cheaper models
	if pctUsed > 70 {
		r.suggestDowngrade(pctUsed)
	}
}

// suggestDowngrade logs routing suggestions when premium budget is running low.
func (r *Router) suggestDowngrade(budgetPct float64) {
	fallback := r.cfg.Routing.FallbackModel

	// Determine routing based on current model usage
	routes := r.suggestRoutes(fallback)

	for _, route := range routes {
		// Log the routing decision
		r.db.Exec(
			"INSERT INTO routing_decisions (from_model, to_model, reason, estimated_savings) VALUES (?, ?, ?, ?)",
			route.FromModel, route.ToModel, route.Reason, route.EstimatedSavings,
		)

		r.db.Exec(
			"INSERT INTO log_events (type, tool, message) VALUES (?, ?, ?)",
			"route", route.FromModel,
			fmt.Sprintf("→ %s (save $%.2f/M tokens): %s", route.ToModel, route.EstimatedSavings, route.Reason),
		)

		notifications.Send(notifications.Alert{
			Level:   "info",
			Title:   "🔀 AgentGuard Routing",
			Message: fmt.Sprintf("%s → %s (%.0f%% budget used)", route.FromModel, route.ToModel, budgetPct),
		})
	}
}

// suggestRoutes returns routing suggestions based on current model costs.
func (r *Router) suggestRoutes(fallback string) []RoutingDecision {
	var routes []RoutingDecision

	// Get recently used models
	rows, err := r.db.Query(`
		SELECT DISTINCT model FROM cost_entries
		WHERE timestamp >= datetime('now', '-1 hour')
		AND model != ''
	`)
	if err != nil {
		return routes
	}
	defer rows.Close()

	for rows.Next() {
		var model string
		rows.Scan(&model)

		currentCost := modelTiers[model]
		if currentCost == 0 {
			continue
		}

		fallbackCost := modelTiers[fallback]
		if fallbackCost == 0 {
			fallbackCost = 0.50 // default fallback cost
		}

		if currentCost > fallbackCost*2 {
			routes = append(routes, RoutingDecision{
				FromModel:        model,
				ToModel:          fallback,
				Reason:           fmt.Sprintf("Budget at %.0f%% — cheaper alternative available", 0.0),
				EstimatedSavings: currentCost - fallbackCost,
			})
		}
	}

	return routes
}

// GetRoutingHistory returns recent routing decisions.
func (r *Router) GetRoutingHistory() []RoutingDecision {
	rows, err := r.db.Query(`
		SELECT from_model, to_model, reason, estimated_savings
		FROM routing_decisions
		ORDER BY timestamp DESC
		LIMIT 20
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var decisions []RoutingDecision
	for rows.Next() {
		var d RoutingDecision
		rows.Scan(&d.FromModel, &d.ToModel, &d.Reason, &d.EstimatedSavings)
		decisions = append(decisions, d)
	}
	return decisions
}

// GetModelCost returns the approximate cost per 1M tokens for a model.
func GetModelCost(model string) float64 {
	if cost, ok := modelTiers[model]; ok {
		return cost
	}
	return 0
}

// getTodaySpend returns total spend since midnight.
func (r *Router) getTodaySpend() float64 {
	today := time.Now().Format("2006-01-02")
	var total float64
	row := r.db.QueryRow(
		"SELECT COALESCE(SUM(cost), 0) FROM cost_entries WHERE DATE(timestamp) = ?",
		today,
	)
	row.Scan(&total)
	return total
}

// getBudget retrieves the budget for a period.
func (r *Router) getBudget(period string) float64 {
	var amount float64
	row := r.db.QueryRow("SELECT amount FROM budgets WHERE period = ?", period)
	err := row.Scan(&amount)
	if err != nil {
		switch period {
		case "daily":
			return r.cfg.Budget.Daily
		case "weekly":
			return r.cfg.Budget.Weekly
		}
		return 0
	}
	return amount
}

// Shutdown gracefully stops the router.
func (r *Router) Shutdown() {
	close(r.stopCh)
}
