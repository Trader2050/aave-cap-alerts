package notify

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TelegramNotifier delivers updates through a Telegram bot.
type TelegramNotifier struct {
	botToken   string
	chatID     string
	httpClient *http.Client
}

// NewTelegramNotifier builds a Telegram notifier with the supplied credentials.
func NewTelegramNotifier(botToken, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		botToken:   botToken,
		chatID:     chatID,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify sends the event payload to the configured chat.
func (t *TelegramNotifier) Notify(ctx context.Context, event SupplyChangeEvent) error {
	message := renderMessage(event)

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%v/sendMessage", t.botToken)
	form := url.Values{}
	form.Set("chat_id", t.chatID)
	form.Set("text", message)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram returned status %s", resp.Status)
	}

	return nil
}

func renderMessage(event SupplyChangeEvent) string {
	var sb strings.Builder
	sb.WriteString("Asset total supply change detected\n")
	sb.WriteString(fmt.Sprintf("Asset: %s (%s)\n", event.AssetName, event.AssetAddress))
	sb.WriteString(fmt.Sprintf("New total supply: %s\n", formatTokens(event.NewTotalSupply)))
	if event.OldTotalSupply != nil {
		sb.WriteString(fmt.Sprintf("Previous total supply: %s\n", formatTokens(event.OldTotalSupply)))
	}
	if event.TargetTotalSupply != nil {
		sb.WriteString(fmt.Sprintf("Target threshold: %s\n", formatTokens(event.TargetTotalSupply)))
	}
	if len(event.TriggerReasons) > 0 {
		sb.WriteString("Reasons:\n")
		for _, reason := range event.TriggerReasons {
			sb.WriteString("- ")
			sb.WriteString(reason)
			sb.WriteString("\n")
		}
	}
	sb.WriteString(fmt.Sprintf("Observed at: %s", event.ObservedAt.UTC().Format(time.RFC3339)))
	return sb.String()
}

func formatTokens(amount *big.Int) string {
	if amount == nil {
		return "n/a"
	}

	digits := amount.String()
	if len(digits) <= 3 {
		return digits
	}

	var parts []string
	for len(digits) > 3 {
		parts = append([]string{digits[len(digits)-3:]}, parts...)
		digits = digits[:len(digits)-3]
	}
	parts = append([]string{digits}, parts...)
	return strings.Join(parts, ",")
}
