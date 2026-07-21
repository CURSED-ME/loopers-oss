package alerting

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/logging"
	"github.com/CURSED-ME/loopers-oss/internal/netutil"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

var alertsDroppedTotal = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "loopers_alerts_dropped_total",
	Help: "Total number of security alerts dropped due to buffer overflow",
})

func init() {
	prometheus.MustRegister(alertsDroppedTotal)
}

type ThresholdConfig struct {
	Percent float64 `mapstructure:"percent"`
	Message string  `mapstructure:"message"`
}

type AlertingConfig struct {
	WebhookURL    string            `mapstructure:"webhook_url"`
	WebhookSecret string            `mapstructure:"webhook_secret"`
	BufferSize    int               `mapstructure:"buffer_size"`
	Thresholds    []ThresholdConfig `mapstructure:"thresholds"`
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
	cfg       AlertingConfig
	client    *http.Client
	rdb       *redis.Client
	ch        chan AlertEvent
	closed    bool
	mu        sync.Mutex
	closeOnce sync.Once
}

func NewAlerter(cfg AlertingConfig, rdb *redis.Client) *Alerter {
	bufSize := cfg.BufferSize
	if bufSize <= 0 {
		bufSize = 10000 // Increased default buffer size to prevent drops (VULN-047)
	}
	a := &Alerter{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DialContext: netutil.SecureDialContext,
			},
		},
		rdb: rdb,
		ch:  make(chan AlertEvent, bufSize),
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
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	select {
	case a.ch <- event:
	default:
		alertsDroppedTotal.Inc()
		logging.Logger.Warn().Msg("Alert channel full, dropping budget exceeded alert")
	}
	a.mu.Unlock()
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

	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	select {
	case a.ch <- event:
	default:
		alertsDroppedTotal.Inc()
		logging.Logger.Warn().Msg("Alert channel full, dropping loop detected alert")
	}
	a.mu.Unlock()
}

func (a *Alerter) TriggerAuthFail(ctx context.Context, requestID, reason string) {
	if a == nil {
		return
	}
	event := NewAuthFailEvent(ctx, requestID, reason, nil)
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	select {
	case a.ch <- event:
	default:
		alertsDroppedTotal.Inc()
		logging.Logger.Warn().Msg("Alert channel full, dropping auth failure alert")
	}
	a.mu.Unlock()
}

func (a *Alerter) TriggerFailClosed(ctx context.Context, requestID, reason string) {
	if a == nil {
		return
	}
	event := NewFailClosedEvent(ctx, requestID, reason, nil)
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	select {
	case a.ch <- event:
	default:
		alertsDroppedTotal.Inc()
		logging.Logger.Warn().Msg("Alert channel full, dropping fail-closed alert")
	}
	a.mu.Unlock()
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
					a.mu.Lock()
					if a.closed {
						a.mu.Unlock()
						continue
					}
					select {
					case a.ch <- event:
					default:
						alertsDroppedTotal.Inc()
						logging.Logger.Warn().Msg("Alert channel full, dropping threshold alert")
					}
					a.mu.Unlock()
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
			if netutil.IsPrivateURL(a.cfg.WebhookURL) && !viper.GetBool("testing.allow_private_urls") {
				logging.Logger.Error().Str("url", a.cfg.WebhookURL).Msg("SECURITY WARNING: Webhook URL resolves to a private IP. Dropping alert.")
				continue
			}

			req, err := http.NewRequest("POST", a.cfg.WebhookURL, bytes.NewReader(payload))
			if err != nil {
				logging.Logger.Error().Err(err).Str("url", a.cfg.WebhookURL).Msg("failed to create webhook request")
				continue
			}
			req.Header.Set("Content-Type", "application/json")

			if a.cfg.WebhookSecret != "" {
				mac := hmac.New(sha256.New, []byte(a.cfg.WebhookSecret))
				mac.Write(payload)
				sig := hex.EncodeToString(mac.Sum(nil))
				req.Header.Set("X-Loopers-Signature", "sha256="+sig)
			}

			resp, err := a.client.Do(req)
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
	if a == nil {
		return
	}
	a.closeOnce.Do(func() {
		a.mu.Lock()
		a.closed = true
		a.mu.Unlock()
		if a.ch != nil {
			close(a.ch)
		}
	})
}
