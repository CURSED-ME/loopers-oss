package loopers.policy

default allow = false

# Allow admin users to do anything
allow {
    input.agent.owner == "admin"
}

# Allow users in 'prod' environment to use gpt-4o
allow {
    input.agent.tags["env"] == "prod"
    input.request.model == "gpt-4o"
}

# General allow for basic models
allow {
    input.request.model == "gpt-3.5-turbo"
}
