#!/usr/bin/env bash
set -euo pipefail

# AgentGuard Installer
# The sentinel that guards your AI budget

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}"
cat << 'EOF'
    ___                   ____
   /   | ____ ____  ____/ / ___/___  ______ __________
  / /| |/ __ `/ _ \/ __  / (_ / __ \/ __ `/ ___/ ___/ \
 / ___ / /_/ /  __/ /_/ / ___/ /_/ / /_/ / /  / /
/_/  |_\__, /\___/\__,_/_/  / .___/\__,_/_/  /_/
      /____/               /_/
EOF
echo -e "${NC}"
echo -e "${GREEN}🔒 AgentGuard Installer${NC}"
echo ""

# Check for Go
check_go() {
    if ! command -v go &> /dev/null; then
        echo -e "${RED}❌ Go is not installed. Install Go 1.21+ first:${NC}"
        echo "   https://go.dev/dl/"
        exit 1
    fi

    GO_VERSION=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')
    echo -e "${GREEN}✓ Go ${GO_VERSION} found${NC}"
}

# Build and install
install_binary() {
    echo -e "${YELLOW}→ Building agentguard...${NC}"
    go build -o agentguard . || { echo -e "${RED}Build failed${NC}"; exit 1; }

    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "$INSTALL_DIR"
    cp agentguard "$INSTALL_DIR/agentguard"
    chmod +x "$INSTALL_DIR/agentguard"
    echo -e "${GREEN}✓ Installed to ${INSTALL_DIR}/agentguard${NC}"

    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        echo -e "${YELLOW}⚠ Add ${INSTALL_DIR} to your PATH:${NC}"
        echo "   export PATH=\"\$HOME/.local/bin:\$PATH\""
    fi
}

# Create config directory
setup_config() {
    CONFIG_DIR="${HOME}/.agentguard"
    mkdir -p "$CONFIG_DIR"

    if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
        cat > "$CONFIG_DIR/config.yaml" << 'YAML'
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
monitored_tools:
  - name: claude
    log_pattern: "~/.claude/logs/*.log"
    cost_regex: 'cost["\s:=]+\$?([\d.]+)'
  - name: aider
    log_pattern: "~/.aider/logs/*.log"
    cost_regex: 'cost["\s:=]+\$?([\d.]+)'
  - name: codex
    log_pattern: "~/.codex/logs/*.log"
    cost_regex: 'cost["\s:=]+\$?([\d.]+)'
  - name: qwen
    log_pattern: "~/.qwen/logs/*.log"
    cost_regex: 'cost["\s:=]+\$?([\d.]+)'
YAML
        echo -e "${GREEN}✓ Config created at ${CONFIG_DIR}/config.yaml${NC}"
    else
        echo -e "${YELLOW}⚠ Config already exists, skipping${NC}"
    fi

    # Create SQLite database
    if [ ! -f "$CONFIG_DIR/agentguard.db" ]; then
        echo -e "${YELLOW}→ Initializing database...${NC}"
        sqlite3 "$CONFIG_DIR/agentguard.db" < database/schema.sql 2>/dev/null || true
        echo -e "${GREEN}✓ Database initialized${NC}"
    fi
}

# Setup systemd service (optional, Linux only)
setup_systemd() {
    if [[ "$(uname)" != "Linux" ]]; then
        echo -e "${YELLOW}⚠ Systemd service only available on Linux${NC}"
        return
    fi

    if ! command -v systemctl &> /dev/null; then
        echo -e "${YELLOW}⚠ systemctl not found, skipping systemd setup${NC}"
        return
    fi

    echo -e "${YELLOW}→ Setting up systemd service...${NC}"

    SERVICE_FILE="${HOME}/.config/systemd/user/agentguard.service"
    mkdir -p "$(dirname "$SERVICE_FILE")"

    cat > "$SERVICE_FILE" << SERVICE
[Unit]
Description=AgentGuard - AI Budget Sentinel
After=network.target

[Service]
Type=simple
ExecStart=${HOME}/.local/bin/agentguard start --foreground
Restart=on-failure
RestartSec=5
Environment=AGENTGUARD_CONFIG=${HOME}/.agentguard/config.yaml

[Install]
WantedBy=default.target
SERVICE

    systemctl --user daemon-reload
    systemctl --user enable agentguard.service
    echo -e "${GREEN}✓ Systemd service installed${NC}"
    echo -e "  Start with: ${CYAN}systemctl --user start agentguard${NC}"
}

# Main
echo ""
check_go
echo ""
install_binary
echo ""
setup_config
echo ""
setup_systemd
echo ""
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo -e "${GREEN}  🔒 AgentGuard installed successfully!${NC}"
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo ""
echo -e "  Quick start:"
echo -e "    ${CYAN}agentguard set-budget daily 10.00${NC}"
echo -e "    ${CYAN}agentguard start${NC}"
echo -e "    ${CYAN}agentguard status${NC}"
echo ""
