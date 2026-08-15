package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseYAML_WithFSM(t *testing.T) {
	yamlData := []byte(`
version: v1
metadata:
  name: fsm-policy
fsm:
  initial_state: UNAUTHENTICATED
  transitions:
    - from: UNAUTHENTICATED
      to: AUTHENTICATED
      trigger: verify_credentials
    - from: AUTHENTICATED
      to: TRANSACTION_ACTIVE
      trigger: start_transaction
rules:
  - name: block-unauthorized-db
    match:
      type: mcp_tool_call
      tool: delete_database
    conditions:
      - field: session.state
        op: not_equals
        value: TRANSACTION_ACTIVE
    action: deny
`)

	card, err := ParseYAML(yamlData)
	assert.NoError(t, err)
	assert.NotNil(t, card.FSM)
	assert.Equal(t, "UNAUTHENTICATED", card.FSM.InitialState)
	assert.Len(t, card.FSM.Transitions, 2)
	assert.Equal(t, "UNAUTHENTICATED", card.FSM.Transitions[0].From)
	assert.Equal(t, "AUTHENTICATED", card.FSM.Transitions[0].To)
	assert.Equal(t, "verify_credentials", card.FSM.Transitions[0].Trigger)

	assert.Len(t, card.Rules, 1)
	assert.Equal(t, "session.state", card.Rules[0].Conditions[0].Field)
	assert.Equal(t, "not_equals", card.Rules[0].Conditions[0].Op)
	assert.Equal(t, "TRANSACTION_ACTIVE", card.Rules[0].Conditions[0].Value)

	regoCode, err := TranspileToRego(card)
	assert.NoError(t, err)
	assert.Contains(t, regoCode, `input.session.state != "TRANSACTION_ACTIVE"`)
}
