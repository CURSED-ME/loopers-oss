package budget

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// BudgetExceededError is returned when a budget limit is reached.
type BudgetExceededError struct {
	WindowName   string
	CurrentSpend float64
	Limit        float64
	Status       string
}

func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf("budget exceeded for %s window: limit %f, current spend %f", e.WindowName, e.Limit, e.CurrentSpend)
}

// CheckAndReserve checks the daily and hourly budgets in Redis and reserves the estimated cost.
// If either check fails, the transaction fails closed.
type WindowInfo struct {
	Name string
	Key  string
	TTL  int
}

func getWindowConfigs(keyHash string, now time.Time) []WindowInfo {
	// Daily
	nextDay := now.AddDate(0, 0, 1).Truncate(24 * time.Hour)
	dailyTTL := int(nextDay.Sub(now).Seconds())

	// Hourly
	nextHour := now.Add(time.Hour).Truncate(time.Hour)
	hourlyTTL := int(nextHour.Sub(now).Seconds())

	// Minute
	nextMinute := now.Add(time.Minute).Truncate(time.Minute)
	minuteTTL := int(nextMinute.Sub(now).Seconds())

	// Weekly (Monday of next week)
	daysToMonday := 8 - int(now.Weekday())
	if now.Weekday() == time.Sunday {
		daysToMonday = 1
	}
	nextWeek := now.AddDate(0, 0, daysToMonday).Truncate(24 * time.Hour)
	weeklyTTL := int(nextWeek.Sub(now).Seconds())
	year, week := now.ISOWeek()

	// Monthly
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	monthlyTTL := int(nextMonth.Sub(now).Seconds())

	configs := []WindowInfo{
		{
			Name: "minute",
			Key:  fmt.Sprintf("loopers:spend:%s:minute:%s", keyHash, now.Format("2006-01-02T15:04")),
			TTL:  minuteTTL,
		},
		{
			Name: "hourly",
			Key:  fmt.Sprintf("loopers:spend:%s:hourly:%s", keyHash, now.Format("2006-01-02T15")),
			TTL:  hourlyTTL,
		},
		{
			Name: "daily",
			Key:  fmt.Sprintf("loopers:spend:%s:daily:%s", keyHash, now.Format("2006-01-02")),
			TTL:  dailyTTL,
		},
		{
			Name: "weekly",
			Key:  fmt.Sprintf("loopers:spend:%s:weekly:%d-W%02d", keyHash, year, week),
			TTL:  weeklyTTL,
		},
		{
			Name: "monthly",
			Key:  fmt.Sprintf("loopers:spend:%s:monthly:%s", keyHash, now.Format("2006-01")),
			TTL:  monthlyTTL,
		},
	}

	for i := range configs {
		if configs[i].TTL <= 0 {
			configs[i].TTL = 1
		}
	}

	return configs
}

// CheckAndReserve checks the configured budgets in Redis and reserves the estimated cost.
// If any check fails, the transaction fails closed and previously reserved windows are rolled back.
func (c *Client) CheckAndReserve(ctx context.Context, keyHash string, estCost float64) error {
	now := time.Now().UTC()
	configKey := fmt.Sprintf("loopers:budget:%s:config", keyHash)
	windows := getWindowConfigs(keyHash, now)

	var checked []WindowInfo
	for _, w := range windows {
		err := c.checkAndReserveWindow(ctx, w.Key, configKey, estCost, w.Name, w.TTL)
		if err != nil {
			// Rollback already reserved windows
			for _, prev := range checked {
				_, _ = c.rdb.IncrByFloat(ctx, prev.Key, -estCost).Result()
			}
			return err
		}
		checked = append(checked, w)
	}

	return nil
}

func (c *Client) checkAndReserveWindow(ctx context.Context, spendKey, configKey string, estCost float64, windowName string, ttlSeconds int) error {
	res, err := checkScript.Run(ctx, c.rdb, []string{spendKey, configKey}, strconv.FormatFloat(estCost, 'f', -1, 64), windowName, ttlSeconds).Result()
	if err != nil {
		return fmt.Errorf("redis error during budget check: %w", err) // fail-closed
	}

	slice, ok := res.([]interface{})
	if !ok || len(slice) < 4 {
		return fmt.Errorf("unexpected response from check script: %v", res)
	}

	allowed, _ := slice[0].(int64)
	if allowed == 0 {
		currentStr, _ := slice[1].(string)
		limitStr, _ := slice[2].(string)
		status, _ := slice[3].(string)

		current, _ := strconv.ParseFloat(currentStr, 64)
		limit, _ := strconv.ParseFloat(limitStr, 64)

		return &BudgetExceededError{
			WindowName:   windowName,
			CurrentSpend: current,
			Limit:        limit,
			Status:       status,
		}
	}

	return nil
}

// Reconcile reconciles the actual spend after request/stream completion.
func (c *Client) Reconcile(ctx context.Context, keyHash string, reservedCost, actualCost float64) error {
	now := time.Now().UTC()
	windows := getWindowConfigs(keyHash, now)

	var lastErr error
	for _, w := range windows {
		err := c.reconcileWindow(ctx, w.Key, reservedCost, actualCost, w.TTL)
		if err != nil {
			lastErr = fmt.Errorf("failed to reconcile %s spend: %w", w.Name, err)
		}
	}

	return lastErr
}

func (c *Client) reconcileWindow(ctx context.Context, spendKey string, reservedCost, actualCost float64, ttlSeconds int) error {
	_, err := reconcileScript.Run(ctx, c.rdb, []string{spendKey}, strconv.FormatFloat(reservedCost, 'f', -1, 64), strconv.FormatFloat(actualCost, 'f', -1, 64), ttlSeconds).Result()
	return err
}

// WindowStatus represents the status of a specific budget window.
type WindowStatus struct {
	Limit        float64 // 0 if not configured
	CurrentSpend float64
}

// GetBudgetStatus returns the limit and current spend for all 5 windows.
func (c *Client) GetBudgetStatus(ctx context.Context, keyHash string) (map[string]WindowStatus, error) {
	now := time.Now().UTC()
	configKey := fmt.Sprintf("loopers:budget:%s:config", keyHash)
	windows := getWindowConfigs(keyHash, now)

	limits, err := c.rdb.HGetAll(ctx, configKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read budget config: %w", err)
	}

	status := make(map[string]WindowStatus)
	for _, w := range windows {
		var limit float64
		if limitStr, exists := limits[w.Name]; exists && limitStr != "" {
			limit, _ = strconv.ParseFloat(limitStr, 64)
		}

		spendStr, _ := c.rdb.Get(ctx, w.Key).Result()
		var spend float64
		if spendStr != "" {
			spend, _ = strconv.ParseFloat(spendStr, 64)
		}

		status[w.Name] = WindowStatus{
			Limit:        limit,
			CurrentSpend: spend,
		}
	}

	return status, nil
}

// CheckAndReserveSession checks session budgets and max steps and reserves estimated spend and step increments.
// Returns (allowed, val1, val2, status, error)
func (c *Client) CheckAndReserveSession(ctx context.Context, sessionID string, estCost float64, inputBudget float64, inputMaxSteps int, ttlSeconds int) (bool, float64, float64, string, error) {
	spendKey := fmt.Sprintf("loopers:session:%s:spend", sessionID)
	budgetKey := fmt.Sprintf("loopers:session:%s:budget", sessionID)
	stepsKey := fmt.Sprintf("loopers:session:%s:steps", sessionID)
	maxStepsKey := fmt.Sprintf("loopers:session:%s:max_steps", sessionID)

	budgetArg := ""
	if inputBudget > 0 {
		budgetArg = strconv.FormatFloat(inputBudget, 'f', -1, 64)
	}
	maxStepsArg := ""
	if inputMaxSteps > 0 {
		maxStepsArg = strconv.Itoa(inputMaxSteps)
	}

	res, err := sessionCheckScript.Run(ctx, c.rdb,
		[]string{spendKey, budgetKey, stepsKey, maxStepsKey},
		strconv.FormatFloat(estCost, 'f', -1, 64),
		budgetArg,
		maxStepsArg,
		ttlSeconds,
	).Result()

	if err != nil {
		return false, 0, 0, "", fmt.Errorf("redis error during session check: %w", err)
	}

	slice, ok := res.([]interface{})
	if !ok || len(slice) < 4 {
		return false, 0, 0, "", fmt.Errorf("unexpected response from session check script: %v", res)
	}

	allowed, _ := slice[0].(int64)
	val1Str, _ := slice[1].(string)
	val2Str, _ := slice[2].(string)
	status, _ := slice[3].(string)

	val1, _ := strconv.ParseFloat(val1Str, 64)
	val2, _ := strconv.ParseFloat(val2Str, 64)

	return allowed == 1, val1, val2, status, nil
}
