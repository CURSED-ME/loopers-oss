package budget

import (
	_ "embed"

	"github.com/redis/go-redis/v9"
)

//go:embed lua_check.lua
var checkScriptSource string

//go:embed lua_reconcile.lua
var reconcileScriptSource string

//go:embed lua_session_check.lua
var sessionCheckScriptSource string

var (
	checkScript        *redis.Script
	reconcileScript    *redis.Script
	sessionCheckScript *redis.Script
)

func init() {
	checkScript = redis.NewScript(checkScriptSource)
	reconcileScript = redis.NewScript(reconcileScriptSource)
	sessionCheckScript = redis.NewScript(sessionCheckScriptSource)
}
