package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/redis/go-redis/v9"
)

type ThresholdConfig struct {
	Percent float64 `mapstructure:"percent"`
	Message string  `mapstructure:"message"`
}

type AlertingConfig struct {
	WebhookURL string            `mapstructure:"webhook_url"`
	Thresholds []ThresholdConfig `mapstructure:"thresholds"`
}

type AlertEvent interface {
	EventType() string
}

type ThresholdAlert struct {
	KeyHash          string  `json:"key_hash"`
	KeyName          string  `json:"key_name"`
	Provider         string  `json:"provider"`
	Window           string  `json:"window"`
	ThresholdPercent int     `json:"threshold_percent"`
	CurrentSpendUSD  float64 `json:"current_spend_usd"`
	BudgetLimitUSD   float64 `json:"budget_limit_usd"`
	Message          string  `json:"message"`
}

type BudgetExceededAlert struct {
	KeyHash               string  `json:"key_hash"`
	KeyName               string  `json:"key_name"`
	Provider              string  `json:"provider"`
	Model                 string  `json:"model"`
	Window                string  `json:"window"`
	CurrentSpendUSD       float64 `json:"current_spend_usd"`
	BudgetLimitUSD        float64 `json:"budget_limit_usd"`
	BlockedRequestCostUSD float64 `json:"blocked_request_cost_usd"`
}

type LoopDetectedAlert struct {
	KeyHash   string `json:"key_hash"`
	KeyName   string `json:"key_name"`
	Provider  string `json:"provider"`
	SessionID string `json:"session_id"`
	Rule      string `json:"rule"`
	Detail    string `json:"detail"`
	Blocked   bool   `json:"blocked"`
}

type Alerter struct {
	cfg    AlertingConfig
	client *http.Client
	rdb    *redis.Client
	ch     chan AlertEvent
}

func NewAlerter(cfg AlertingConfig, rdb *redis.Client) *Alerter {
	a := &Alerter{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		rdb:    rdb,
		ch:     make(chan AlertEvent, 100),
	}
	go a.worker()
	return a
}

func (a *Alerter) TriggerBlockAlert(ctx context.Context, requestID, keyHash, keyName, provider, model, window string, currentSpend, limit, blockedCost float64) {
	if a == nil {
		return
	}
	details := &BudgetExceededAlert{
		KeyHash:               keyHash,
		KeyName:               keyName,
		Provider:              provider,
		Model:                 model,
		Window:                window,
		CurrentSpendUSD:       currentSpend,
		BudgetLimitUSD:        limit,
		BlockedRequestCostUSD: blockedCost,
	}
	event := NewBudgetBlockEvent(ctx, requestID, details)
	select {
	case a.ch <- event:
	default:
		logging.Logger.Warn().Msg("Alert channel full, dropping budget exceeded alert")
	}
}

func (a *Alerter) TriggerLoopAlert(ctx context.Context, requestID, keyHash, keyName, provider, sessionID, rule, detail string, blocked bool) {
	if a == nil {
		return
	}

	details := &LoopDetectedAlert{
		KeyHash:   keyHash,
		KeyName:   keyName,
		Provider:  provider,
		SessionID: sessionID,
		Rule:      rule,
		Detail:    detail,
		Blocked:   blocked,
	}

	var event *SecurityEventEnvelope
	if blocked {
		event = NewLoopBlockEvent(ctx, requestID, details)
	} else {
		event = NewLoopWarnEvent(ctx, requestID, details)
	}

	select {
	case a.ch <- event:
	default:
		logging.Logger.Warn().Msg("Alert channel full, dropping loop detected alert")
	}
}

func (a *Alerter) TriggerAuthFail(ctx context.Context, requestID, reason string) {
	if a == nil {
		return
	}
	event := NewAuthFailEvent(ctx, requestID, reason, nil)
	select {
	case a.ch <- event:
	default:
		logging.Logger.Warn().Msg("Alert channel full, dropping auth failure alert")
	}
}

func (a *Alerter) TriggerFailClosed(ctx context.Context, requestID, reason string) {
	if a == nil {
		return
	}
	event := NewFailClosedEvent(ctx, requestID, reason, nil)
	select {
	case a.ch <- event:
	default:
		logging.Logger.Warn().Msg("Alert channel full, dropping fail-closed alert")
	}
}

func (a *Alerter) TriggerThresholdAlerts(ctx context.Context, requestID, keyHash, keyName, provider string, currentSpends map[string]float64, limits map[string]float64) {
	if a == nil || len(a.cfg.Thresholds) == 0 {
		return
	}

	now := time.Now().UTC()
	for windowName, limit := range limits {
		if limit <= 0 {
			continue
		}
		spend := currentSpends[windowName]
		pct := (spend / limit) * 100.0

		for _, t := range a.cfg.Thresholds {
			if pct >= t.Percent {
				// Check and set deduplication key in Redis
				dedupKey := a.getDedupKey(keyHash, windowName, now, int(t.Percent))
				ttl := a.getDedupTTL(windowName, now)

				// Set NX
				set, err := a.rdb.SetNX(ctx, dedupKey, "fired", ttl).Result()
				if err != nil {
					logging.Logger.Error().Err(err).Msg("failed to set alert deduplication key in Redis")
					continue
				}
				if set {
					// Fire alert
					details := &ThresholdAlert{
						KeyHash:          keyHash,
						KeyName:          keyName,
						Provider:         provider,
						Window:           windowName,
						ThresholdPercent: int(t.Percent),
						CurrentSpendUSD:  spend,
						BudgetLimitUSD:   limit,
						Message:          t.Message,
					}
					event := NewBudgetThresholdEvent(ctx, requestID, details)
					select {
					case a.ch <- event:
					default:
						logging.Logger.Warn().Msg("Alert channel full, dropping threshold alert")
					}
				}
			}
		}
	}
}

func (a *Alerter) getDedupKey(keyHash, window string, now time.Time, threshold int) string {
	switch window {
	case "minute":
		return fmt.Sprintf("loopers:alert:%s:minute:%s:%d", keyHash, now.Format("2006-01-02T15:04"), threshold)
	case "hourly":
		return fmt.Sprintf("loopers:alert:%s:hourly:%s:%d", keyHash, now.Format("2006-01-02T15"), threshold)
	case "daily":
		return fmt.Sprintf("loopers:alert:%s:daily:%s:%d", keyHash, now.Format("2006-01-02"), threshold)
	case "weekly":
		year, week := now.ISOWeek()
		return fmt.Sprintf("loopers:alert:%s:weekly:%d-W%02d:%d", keyHash, year, week, threshold)
	case "monthly":
		return fmt.Sprintf("loopers:alert:%s:monthly:%s:%d", keyHash, now.Format("2006-01"), threshold)
	default:
		return fmt.Sprintf("loopers:alert:%s:%s:%d", keyHash, window, threshold)
	}
}

func (a *Alerter) getDedupTTL(window string, now time.Time) time.Duration {
	var next time.Time
	switch window {
	case "minute":
		next = now.Add(time.Minute).Truncate(time.Minute)
	case "hourly":
		next = now.Add(time.Hour).Truncate(time.Hour)
	case "daily":
		next = now.AddDate(0, 0, 1).Truncate(24 * time.Hour)
	case "weekly":
		daysToMonday := 8 - int(now.Weekday())
		if now.Weekday() == time.Sunday {
			daysToMonday = 1
		}
		next = now.AddDate(0, 0, daysToMonday).Truncate(24 * time.Hour)
	case "monthly":
		next = time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	default:
		return 24 * time.Hour
	}
	dur := next.Sub(now)
	if dur <= 0 {
		return time.Second
	}
	return dur
}

func (a *Alerter) worker() {
	for event := range a.ch {
		payload, err := json.Marshal(event)
		if err != nil {
			logging.Logger.Error().Err(err).Msg("failed to marshal alert event")
			continue
		}

		// Always emit to stdout
		fmt.Println(string(payload))

		if a.cfg.WebhookURL != "" {
			resp, err := a.client.Post(a.cfg.WebhookURL, "application/json", bytes.NewReader(payload))
			if err != nil {
				logging.Logger.Error().Err(err).Str("url", a.cfg.WebhookURL).Msg("failed to deliver webhook alert")
				continue
			}
			resp.Body.Close()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				logging.Logger.Error().Int("status", resp.StatusCode).Str("url", a.cfg.WebhookURL).Msg("webhook host returned non-2xx response")
			}
		}
	}
}

func (a *Alerter) Close() {
	if a != nil && a.ch != nil {
		close(a.ch)
	}
}
