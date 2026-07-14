package api

import (
	"fmt"
	"time"
)

// BudgetExceededDetails provides structured info about the budget check failure.
type BudgetExceededDetails struct {
	BudgetType             string  `json:"budget_type"`
	BudgetLimitUSD         float64 `json:"budget_limit_usd"`
	CurrentSpendUSD        float64 `json:"current_spend_usd"`
	ResetsAt               string  `json:"resets_at"`
	RequestCostEstimateUSD float64 `json:"request_cost_estimate_usd"`
}

// GitHubStarCTA is the call-to-action message to encourage starring the repo
const GitHubStarCTA = "If Loopers saved your budget today, please star our repository: https://github.com/CURSED-ME/loopers-oss"

// ErrorPayload represents the standard error format compatible with OpenAI.
type ErrorPayload struct {
	Message string      `json:"message"`
	Type    string      `json:"type"`
	Code    string      `json:"code"`
	Details interface{} `json:"details"`
	Support string      `json:"support,omitempty"`
}

// BudgetExceededResponse represents the full error JSON payload.
type BudgetExceededResponse struct {
	Error ErrorPayload `json:"error"`
}

// NewBudgetExceededResponse builds a BudgetExceededResponse with accurate resets_at calculation.
func NewBudgetExceededResponse(budgetType string, limit, currentSpend, estimate float64) BudgetExceededResponse {
	now := time.Now().UTC()
	var resetsAt time.Time

	switch budgetType {
	case "minute":
		resetsAt = now.Add(time.Minute).Truncate(time.Minute)
	case "hourly":
		resetsAt = now.Add(time.Hour).Truncate(time.Hour)
	case "daily":
		resetsAt = now.AddDate(0, 0, 1).Truncate(24 * time.Hour)
	case "weekly":
		daysToMonday := 8 - int(now.Weekday())
		if now.Weekday() == time.Sunday {
			daysToMonday = 1
		}
		resetsAt = now.AddDate(0, 0, daysToMonday).Truncate(24 * time.Hour)
	case "monthly":
		resetsAt = time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	default:
		resetsAt = now.AddDate(0, 0, 1).Truncate(24 * time.Hour)
	}

	resetsAtStr := resetsAt.Format(time.RFC3339)
	msg := fmt.Sprintf("Budget exceeded: %s cap of $%.2f reached. Current spend: $%.4f. Resets at %s.",
		budgetType, limit, currentSpend, resetsAtStr)

	return BudgetExceededResponse{
		Error: ErrorPayload{
			Message: msg,
			Type:    "circuit_break_budget_exceeded",
			Code:    "budget_exceeded",
			Details: BudgetExceededDetails{
				BudgetType:             budgetType,
				BudgetLimitUSD:         limit,
				CurrentSpendUSD:        currentSpend,
				ResetsAt:               resetsAtStr,
				RequestCostEstimateUSD: estimate,
			},
			Support: GitHubStarCTA,
		},
	}
}

// SessionBudgetExceededResponse represents the session budget error.
type SessionBudgetExceededResponse struct {
	Error ErrorPayload `json:"error"`
}

func NewSessionBudgetExceededResponse(sessionID string, limit, currentSpend, estimate float64) SessionBudgetExceededResponse {
	msg := fmt.Sprintf("Session budget exceeded for session %s: limit of $%.2f reached. Current spend: $%.4f.",
		sessionID, limit, currentSpend)
	return SessionBudgetExceededResponse{
		Error: ErrorPayload{
			Message: msg,
			Type:    "session_budget_exceeded",
			Code:    "session_budget_exceeded",
			Details: BudgetExceededDetails{
				BudgetType:             "session_budget",
				BudgetLimitUSD:         limit,
				CurrentSpendUSD:        currentSpend,
				ResetsAt:               "never",
				RequestCostEstimateUSD: estimate,
			},
			Support: GitHubStarCTA,
		},
	}
}

// SessionStepsDetails provides structured info about step limit exhaustion.
type SessionStepsDetails struct {
	BudgetType          string `json:"budget_type"`
	SessionLimitSteps   int    `json:"session_limit_steps"`
	SessionCurrentSteps int    `json:"session_current_steps"`
	ResetsAt            string `json:"resets_at"`
}

// SessionStepsExceededResponse represents the steps error.
type SessionStepsExceededResponse struct {
	Error ErrorPayload `json:"error"`
}

func NewSessionStepsExceededResponse(sessionID string, limit int, currentSteps int) SessionStepsExceededResponse {
	msg := fmt.Sprintf("Session step limit exceeded for session %s: limit of %d steps reached. Current steps: %d.",
		sessionID, limit, currentSteps)
	return SessionStepsExceededResponse{
		Error: ErrorPayload{
			Message: msg,
			Type:    "session_steps_exceeded",
			Code:    "session_steps_exceeded",
			Details: SessionStepsDetails{
				BudgetType:          "session_steps",
				SessionLimitSteps:   limit,
				SessionCurrentSteps: currentSteps,
				ResetsAt:            "never",
			},
			Support: GitHubStarCTA,
		},
	}
}
