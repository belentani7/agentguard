# 🔒 AgentGuard

**The firewall for your AI budget.**

AgentGuard is a daemon that monitors and enforces spending limits for AI coding tools. Set daily/weekly budgets, auto-pause agents when limits hit, and route overflow to cheaper models — all in real time.

## Features

- 📊 **Real-time monitoring** — Watch all AI CLI tools (Claude, Aider, Codex, Qwen) from a single dashboard
- 💰 **Budget enforcement** — Set daily and weekly spending limits with auto-pause at 100%
- 🔔 **Smart alerts** — Desktop notifications at 80%, 95%, and 100% thresholds
- 🔀 **Smart routing** — Automatically route overflow traffic to cheaper models when premium budget is exhausted
- 📈 **Spend forecasting** — `agentguard --predict` forecasts monthly spend based on patterns
- 🛡️ **Circuit breaker** — SIGSTOP/SIGCONT for graceful agent pause/resume

## Install

```bash
# From source
git clone https://github.com/belentani7/agentguard
cd agentguard
go build -o agentguard .

# Or via go install
go install github.com/belentani7/agentguard@latest
```

### Quick Install (Linux/Mac)

```bash
curl -fsSL https://raw.githubusercontent.com/belentani7/agentguard/main/install.sh | bash
```

## Quick Start

```bash
# Set your daily budget ($10/day)
agentguard set-budget daily 10.00

# Set weekly budget ($50/week)
agentguard set-budget weekly 50.00

# Start the sentinel
agentguard start

# Check status
agentguard status

# View logs
agentguard logs

# Manually pause all agents
agentguard pause

# Stop the daemon
agentguard stop
```

## Dashboard

```bash
agentguard status
```

Shows:
- Real-time spend graph
- Breakdown by tool (Claude, Aider, Codex, Qwen)
- Budget remaining (daily/weekly)
- Active agents and their PIDs
- Recent activity log
- Forecasted monthly spend

## Configuration

Edit `~/.agentguard/config.yaml`:

```yaml
budget:
  daily: 10.00
  weekly: 50.00
alerts:
  thresholds: [80, 95, 100]
  email: ""
  slack_webhook: ""
routing:
  fallback_model: qwen
  quality_threshold: 0.8
```

## Architecture

```
agentguard
├── daemon/
│   ├── monitor.go    # File watchers, log parsing, cost aggregation
│   ├── enforcer.go   # Budget checks, notifications, SIGSTOP/SIGCONT
│   └── router.go     # Smart model routing, fallback logic
├── dashboard/
│   └── tui.go        # Bubble Tea TUI dashboard
├── notifications/
│   └── system.go     # Desktop, email, Slack notifications
├── config/
│   └── default.yaml  # Default configuration
├── database/
│   └── schema.sql    # SQLite schema
└── main.go           # CLI entry point
```

## How It Works

1. **Monitor** — Watches log files from AI tools (`~/.claude/logs`, `~/.aider/logs`, etc.)
2. **Parse** — Extracts token usage and cost data from each tool's output format
3. **Aggregate** — Stores normalized cost data in SQLite
4. **Enforce** — Checks running totals against configured budgets
5. **Act** — Sends notifications, pauses agents (SIGSTOP), or routes to cheaper models
6. **Resume** — Automatically resumes agents when budget resets (next day/week)

## The Sentinel

The daemon is the **Sentinel** — always watching, always protecting. It learns your spending patterns over time and can forecast your monthly burn rate.

```bash
# Hidden mode: predict monthly spend
agentguard --predict
```

## License

MIT
