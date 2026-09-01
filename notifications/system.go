package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// Alert represents a notification to send.
type Alert struct {
	Level   string // "info", "warning", "high", "critical"
	Title   string
	Message string
}

// Config for notification delivery.
type Config struct {
	Email        string
	SlackWebhook string
}

var notifConfig Config

// Configure sets up notification channels.
func Configure(cfg Config) {
	notifConfig = cfg
}

// Send dispatches a notification through all configured channels.
func Send(alert Alert) {
	// Always try desktop notification
	sendDesktop(alert)

	// Terminal bell for critical alerts
	if alert.Level == "critical" {
		fmt.Print("\a")
	}

	// Slack webhook if configured
	if notifConfig.SlackWebhook != "" {
		sendSlack(alert)
	}

	// Email if configured
	if notifConfig.Email != "" {
		sendEmail(alert)
	}
}

// sendDesktop sends a desktop notification.
func sendDesktop(alert Alert) {
	switch runtime.GOOS {
	case "linux":
		urgency := "normal"
		switch alert.Level {
		case "critical":
			urgency = "critical"
		case "high":
			urgency = "normal"
		case "warning":
			urgency = "normal"
		}

		exec.Command("notify-send",
			"-u", urgency,
			"-i", "security-high",
			"--expire-time=10000",
			alert.Title,
			alert.Message,
		).Start()

	case "darwin":
		script := fmt.Sprintf(
			`display notification "%s" with title "%s"`,
			alert.Message, alert.Title,
		)
		exec.Command("osascript", "-e", script).Start()

	case "windows":
		// PowerShell toast notification
		script := fmt.Sprintf(`
			[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
			[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
			$template = @"
<toast>
    <visual>
        <binding template="ToastText02">
            <text id="1">%s</text>
            <text id="2">%s</text>
        </binding>
    </visual>
</toast>
"@
			$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
			$xml.LoadXml($template)
			$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
			[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("AgentGuard").Show($toast)
		`, alert.Title, alert.Message)

		exec.Command("powershell", "-Command", script).Start()
	}
}

// sendSlack sends a notification to a Slack webhook.
func sendSlack(alert Alert) {
	color := "#36a64f" // green
	switch alert.Level {
	case "critical":
		color = "#ff0000"
	case "high":
		color = "#ff6600"
	case "warning":
		color = "#ffcc00"
	}

	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color":  color,
				"title":  alert.Title,
				"text":   alert.Message,
				"ts":     time.Now().Unix(),
				"footer": "AgentGuard 🔒",
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(notifConfig.SlackWebhook, "application/json", bytes.NewReader(data))
	if err != nil {
		return
	}
	resp.Body.Close()
}

// sendEmail sends a notification via email (requires sendmail or msmtp).
func sendEmail(alert Alert) {
	if notifConfig.Email == "" {
		return
	}

	subject := fmt.Sprintf("[AgentGuard] %s", alert.Title)
	body := fmt.Sprintf("AgentGuard Alert\n\nLevel: %s\n%s\n\n%s\n",
		alert.Level, alert.Title, alert.Message)

	// Try msmtp first, then sendmail
	for _, mailer := range []string{"msmtp", "sendmail"} {
		cmd := exec.Command(mailer, notifConfig.Email)
		cmd.Stdin = bytes.NewReader([]byte(fmt.Sprintf("Subject: %s\n\n%s", subject, body)))
		if err := cmd.Run(); err == nil {
			return
		}
	}
}
