package budget

import (
	_ "embed"

	"github.com/redis/go-redis/v9"
)

//go:embed lua_check.lua
var checkScriptSource string

//go:embed lua_check_all.lua
var checkAllScriptSource string

//go:embed lua_reconcile.lua
var reconcileScriptSource string

//go:embed lua_reconcile_all.lua
var reconcileAllScriptSource string

//go:embed lua_session_check.lua
var sessionCheckScriptSource string

//go:embed lua_lease_acquire.lua
var leaseAcquireScriptSource string

//go:embed lua_lease_heartbeat.lua
var leaseHeartbeatScriptSource string

//go:embed lua_lease_reclaim.lua
var leaseReclaimScriptSource string

var (
	checkScript          *redis.Script
	checkAllScript       *redis.Script
	reconcileScript      *redis.Script
	reconcileAllScript   *redis.Script
	sessionCheckScript   *redis.Script
	leaseAcquireScript   *redis.Script
	leaseHeartbeatScript *redis.Script
	leaseReclaimScript   *redis.Script
)

func init() {
	checkScript = redis.NewScript(checkScriptSource)
	checkAllScript = redis.NewScript(checkAllScriptSource)
	reconcileScript = redis.NewScript(reconcileScriptSource)
	reconcileAllScript = redis.NewScript(reconcileAllScriptSource)
	sessionCheckScript = redis.NewScript(sessionCheckScriptSource)
	leaseAcquireScript = redis.NewScript(leaseAcquireScriptSource)
	leaseHeartbeatScript = redis.NewScript(leaseHeartbeatScriptSource)
	leaseReclaimScript = redis.NewScript(leaseReclaimScriptSource)
}
