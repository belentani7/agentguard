package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/belentani7/agentguard/daemon"
	"github.com/belentani7/agentguard/dashboard"
	"github.com/belentani7/agentguard/notifications"
)

const version = "0.1.0"

var configDir string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot determine home directory: %v\n", err)
		os.Exit(1)
	}
	configDir = filepath.Join(home, ".agentguard")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	switch cmd {
	case "start":
		cmdStart()
	case "stop":
		cmdStop()
	case "status":
		cmdStatus()
	case "set-budget":
		cmdSetBudget()
	case "logs":
		cmdLogs()
	case "pause":
		cmdPause()
	case "resume":
		cmdResume()
	case "--predict":
		cmdPredict()
	case "--version", "-v":
		fmt.Printf("agentguard v%s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	case "--help", "-h", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(asciiGuard())
	fmt.Printf(`
🔒 AgentGuard v%s — The AI Budget Sentinel

Usage:
  agentguard <command> [options]

Commands:
  start                  Start the sentinel daemon
  stop                   Stop the sentinel daemon
  status                 Show real-time dashboard
  set-budget <period> <amount>
                         Set spending limit (daily|weekly)
  logs                   Show recent activity
  pause                  Manually pause all monitored agents
  resume                 Resume all paused agents
  --predict              Forecast monthly spend (hidden)
  --version, -v          Show version
  --help, -h             Show this help

Examples:
  agentguard set-budget daily 10.00
  agentguard set-budget weekly 50.00
  agentguard start
  agentguard status

`, version)
}

// cmdStart starts the sentinel daemon.
func cmdStart() {
	foreground := false
	for _, arg := range os.Args[2:] {
		if arg == "--foreground" || arg == "-f" {
			foreground = true
		}
	}

	// Check if already running
	pidFile := filepath.Join(configDir, "agentguard.pid")
	if pid, err := os.ReadFile(pidFile); err == nil {
		pidInt, _ := strconv.Atoi(strings.TrimSpace(string(pid)))
		if processExists(pidInt) {
			fmt.Printf("🔒 Sentinel already running (PID %d)\n", pidInt)
			fmt.Println("   Use 'agentguard stop' first to restart.")
			return
		}
	}

	if foreground {
		runDaemon()
		return
	}

	// Fork into background
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot determine executable path: %v\n", err)
		os.Exit(1)
	}

	logFile, err := os.OpenFile(filepath.Join(configDir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot open log file: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(executable, "start", "--foreground")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = daemon.SetSysProcAttr()

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to start daemon: %v\n", err)
		os.Exit(1)
	}

	// Write PID file
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write PID file: %v\n", err)
	}

	fmt.Printf("🔒 Sentinel awakened (PID %d)\n", cmd.Process.Pid)
	fmt.Println("   The guard is watching. Use 'agentguard status' to check.")
}

// cmdStop stops the sentinel daemon.
func cmdStop() {
	pidFile := filepath.Join(configDir, "agentguard.pid")
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("🔒 Sentinel is not running.")
		return
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		fmt.Println("🔒 Sentinel is not running (corrupt PID file).")
		os.Remove(pidFile)
		return
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("🔒 Sentinel is not running.")
		os.Remove(pidFile)
		return
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Printf("🔒 Sentinel was not running (error: %v)\n", err)
		os.Remove(pidFile)
		return
	}

	// Wait for graceful shutdown (up to 10 seconds)
	for i := 0; i < 50; i++ {
		time.Sleep(200 * time.Millisecond)
		if !processExists(pid) {
			break
		}
	}

	// Force kill if still running
	if processExists(pid) {
		proc.Kill()
	}

	os.Remove(pidFile)
	fmt.Println("🔒 Sentinel has retired. Your budget is safe.")
}

// cmdStatus shows the TUI dashboard.
func cmdStatus() {
	db := openDB()
	defer db.Close()

	cfg := loadConfig()
	dashboard.Run(db, cfg)
}

// cmdSetBudget sets a spending limit.
func cmdSetBudget() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: agentguard set-budget <daily|weekly> <amount>")
		fmt.Println("Example: agentguard set-budget daily 10.00")
		os.Exit(1)
	}

	period := os.Args[2]
	if period != "daily" && period != "weekly" {
		fmt.Fprintf(os.Stderr, "error: period must be 'daily' or 'weekly', got '%s'\n", period)
		os.Exit(1)
	}

	amount, err := strconv.ParseFloat(os.Args[3], 64)
	if err != nil || amount <= 0 {
		fmt.Fprintf(os.Stderr, "error: invalid amount '%s'\n", os.Args[3])
		os.Exit(1)
	}

	db := openDB()
	defer db.Close()

	daemon.SetBudget(db, period, amount)

	periodLabel := "daily"
	if period == "weekly" {
		periodLabel = "weekly"
	}
	fmt.Printf("🔒 %s budget set to $%.2f\n", strings.Title(periodLabel), amount)

	// Notify running daemon to reload config
	notifyDaemonReload()
}

// cmdLogs shows recent activity.
func cmdLogs() {
	db := openDB()
	defer db.Close()

	entries := daemon.RecentLogs(db, 25)
	if len(entries) == 0 {
		fmt.Println("No activity recorded yet. Start the sentinel with 'agentguard start'.")
		return
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  🔒 AgentGuard — Recent Activity")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()

	for _, e := range entries {
		icon := "📊"
		switch e.Type {
		case "alert":
			icon = "🔔"
		case "pause":
			icon = "⏸️"
		case "resume":
			icon = "▶️"
		case "route":
			icon = "🔀"
		}
		fmt.Printf("  %s [%s] %-8s %s\n", icon, e.Timestamp.Format("15:04:05"), e.Tool, e.Message)
	}
	fmt.Println()
}

// cmdPause manually pauses all agents.
func cmdPause() {
	db := openDB()
	defer db.Close()

	count := daemon.PauseAllAgents(db)
	fmt.Printf("⏸️  Paused %d agent(s). Budget shield active.\n", count)

	notifications.Send(notifications.Alert{
		Level:   "info",
		Title:   "AgentGuard",
		Message: fmt.Sprintf("Manually paused %d agent(s)", count),
	})
}

// cmdResume resumes all paused agents.
func cmdResume() {
	db := openDB()
	defer db.Close()

	count := daemon.ResumeAllAgents(db)
	fmt.Printf("▶️  Resumed %d agent(s).\n", count)

	notifications.Send(notifications.Alert{
		Level:   "info",
		Title:   "AgentGuard",
		Message: fmt.Sprintf("Resumed %d agent(s)", count),
	})
}

// cmdPredict forecasts monthly spend (hidden feature).
func cmdPredict() {
	db := openDB()
	defer db.Close()

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  🔮 AgentGuard — Spend Oracle")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()

	forecast := daemon.PredictMonthlySpend(db)

	fmt.Printf("  📅 Days tracked:       %d\n", forecast.DaysTracked)
	fmt.Printf("  💰 Avg daily spend:    $%.2f\n", forecast.AvgDailySpend)
	fmt.Printf("  📈 Predicted monthly:  $%.2f\n", forecast.PredictedMonthly)
	fmt.Printf("  📊 Peak day:           $%.2f (%s)\n", forecast.PeakDaySpend, forecast.PeakDayDate)
	fmt.Printf("  📉 Lowest day:         $%.2f (%s)\n", forecast.LowDaySpend, forecast.LowDayDate)
	fmt.Println()

	if forecast.PredictedMonthly > forecast.MonthlyBudget {
		fmt.Printf("  ⚠️  WARNING: Predicted spend ($%.2f) exceeds monthly budget ($%.2f)\n",
			forecast.PredictedMonthly, forecast.MonthlyBudget)
		fmt.Printf("     Consider lowering daily usage or increasing budget.\n")
	} else {
		fmt.Printf("  ✅ Predicted spend is within budget ($%.2f / $%.2f)\n",
			forecast.PredictedMonthly, forecast.MonthlyBudget)
	}
	fmt.Println()
}

// runDaemon runs the daemon in the foreground (for systemd or debugging).
func runDaemon() {
	os.MkdirAll(configDir, 0755)

	db := openDB()
	defer db.Close()

	cfg := loadConfig()

	// Write PID file
	pidFile := filepath.Join(configDir, "agentguard.pid")
	os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	defer os.Remove(pidFile)

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start daemon components
	mon := daemon.NewMonitor(db, cfg)
	enforcer := daemon.NewEnforcer(db, cfg)
	router := daemon.NewRouter(db, cfg)

	fmt.Println("🔒 Sentinel is watching...")

	// Main loop
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			costs := mon.Scan()
			enforcer.Check(costs)
			router.Evaluate(costs)

		case sig := <-sigCh:
			fmt.Printf("\n🔒 Received %s, sentinel retiring...\n", sig)
			mon.Shutdown()
			enforcer.Shutdown()
			router.Shutdown()
			return
		}
	}
}

// processExists checks if a process with the given PID is running.
func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// openDB opens the SQLite database.
func openDB() *daemon.DB {
	dbPath := filepath.Join(configDir, "agentguard.db")
	return daemon.OpenDB(dbPath)
}

// loadConfig loads the YAML configuration.
func loadConfig() daemon.Config {
	cfgPath := filepath.Join(configDir, "config.yaml")
	return daemon.LoadConfig(cfgPath)
}

// notifyDaemonReload sends SIGHUP to the running daemon to reload config.
func notifyDaemonReload() {
	pidFile := filepath.Join(configDir, "agentguard.pid")
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return
	}
	if sigHUP != nil {
		sendSignal(pid, sigHUP)
	} else {
		// Fallback: write reload flag file
		os.WriteFile(filepath.Join(configDir, "reload.flag"), []byte("1"), 0644)
	}
}

// asciiGuard returns the ASCII art shield.
func asciiGuard() string {
	data, err := os.ReadFile(filepath.Join(configDir, "..", "projects", "agentguard", "ascii", "guard.txt"))
	if err == nil {
		return string(data)
	}
	return `
    ___                   ____
   /   | ____ ____  ____/ / ___/___  ______ __________
  / /| |/ __ ` + "`" + `/ _ \/ __  / (_ / __ \/ __ ` + "`" + `/ ___/ ___/ \
 / ___ / /_/ /  __/ /_/ / ___/ /_/ / /_/ / /  / /
/_/  |_\__, /\___/\__,_/_/  / .___/\__,_/_/  /_/
      /____/               /_/
`
}

// readStdinLine reads a single line from stdin (for interactive prompts).
func readStdinLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}
