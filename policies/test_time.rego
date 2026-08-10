package loopers.policy

default allow = false

allow {
    # Only allow during business hours (UTC)
    # Using OPA's built-in time functions because Loopers doesn't pass input.hour_utc
    [hour, minute, second] := time.clock(time.now_ns())
    hour >= 17
    hour <= 20
}

deny[reason] {
    not allow
    reason := "Rego Policy: Access is only allowed during business hours (17:00 - 20:00 UTC)."
}
