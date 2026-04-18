# SpecStar Skill Package — Go AWS Lambda Microservice (Hexagonal)

This package defines a SpecStar skill for building or modifying a Go microservice whose compute boundary is AWS Lambda.

It is designed for services that expose one or both inbound protocols:

- API Gateway HTTP API
- Amazon SQS

The package treats Lambda as the compute host and keeps business logic isolated from transport and infrastructure concerns.

## Package contents

- `SKILL.md` — the skill contract used by the agent
- `skill.manifest.yaml` — skill metadata, routing, dependencies, and defaults
- `docs/architecture.md` — architecture guidance and boundaries
- `docs/task-routing.md` — when to use this skill and which trigger mode fits
- `checklists/done-when.md` — completion criteria
- `examples/` — starter examples for HTTP, SQS, bootstrap, domain, application, and outbound ports/adapters

## Intended companion instructions

This skill is meant to be mounted together with generic cross-cutting instructions such as:

- cleanup
- fail-fast
- testing
- error handling
- logging
- observability

## Main stance

- Keep Lambda handlers thin.
- Keep AWS event types out of domain and application layers.
- Keep business logic reusable across trigger types.
- Treat HTTP and SQS as inbound adapters.
- Treat AWS SDK integrations and external services as outbound adapters.
