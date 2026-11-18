# Plan v1

## Waves

### Wave 1: format and health check
| id | role | language | model - reason | job | owns | must not touch | tests (test files per agent) | claims |
|---|---|---|---|---|---|---|---|---|
| W1-domain | domain-engineer | go | sonnet - small rule | FormatCents | internal/domain/pricing/** | internal/adapters/** | go test -race ./internal/domain/pricing/... | C1 |
| W1-http | api-engineer | go | sonnet - one route | GET /healthz | internal/adapters/inbound/httpapi/** | internal/domain/** | go test -race ./internal/adapters/inbound/httpapi/... | C1 |
