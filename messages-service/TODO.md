# Messages Service Gaps and Next Steps

This repository contains only a thin slice of the desired functionality. Below is an audit of what is missing or needs correction to match the target behavior described in the project brief.

## Config and bootstrapping
- The config schema in `internal/config/config.go` expects sections `http_server`, `kafka`, `postgresql`, `retries`, and `org`, but `configs/messages-service.yaml` defines unrelated keys (`server`, `kafka.group_id`, `database`, etc.) and even has a malformed `org.file_path` line without a newline. The app will fail to start with the provided file and defaults are unlikely sufficient. Align the YAML structure with the Go struct or vice versa; add validation and defaults where necessary.
- `logger.New` exists but is unused in `cmd/main.go`; logging is hard-coded to JSON. Decide on a single logger initialization path and honor the environment.
- No graceful shutdown: the HTTP server and Kafka/Postgres clients are not closed on interrupt. Add signal handling and context cancellation.

## Persistence
- There is no database schema or migrations for the `mails` table and related columns (`classification`, `model_answer`, `failed_reason`, timestamps, `is_approved`, `assistant_response`, etc.). Introduce migrations and keep them in-repo.
- Repository methods cover only a subset of required operations: missing list of processed messages, approval flag updates, and storing assistant responses. Add queries and indexes to support operator endpoints.
- Retries rely on `attempts` but `CreateMail` always inserts `attempts=0` and never resets on success; ensure status/attempt consistency and consider optimistic locking to avoid lost updates.

## HTTP API surface
- Only `/process` and `/validate_processed_message` handlers are implemented. The operator endpoints described in the brief are absent: `getProcessedMessages`, `approveMessage`, and `add-assistant-response`. Add routing, DTOs, validation, and responses for these.
- Responses are empty body with status codes; add structured JSON error/success payloads for observability and client ergonomics.
- Request validation is minimal (checks only a few fields). Add stricter validation (email formats, timestamps, payload shape) and return meaningful errors.

## Kafka integration
- The service writes to Kafka but never consumes anything. If Kafka should be the entrypoint for LLM results or other events, add consumer support with configurable group IDs, timeouts, and retry/backoff. At minimum, ensure producer config honors `Producer.Timeout` and expose metrics.
- `configs/messages-service.yaml` includes `group_id` and `consume_timeout_ms` that are unused; decide whether to support them or drop from config.

## Domain logic gaps
- `/validate_processed_message` performs only superficial validation of the LLM result. Implement structural validation (e.g., JSON schema) and content checks before accepting data.
- Handling of hierarchy data (`configs/hierarchy.json`) is not implemented. Load and expose it in the service to drive routing/notifications/approvals as required by the project description.
- Status lifecycle is incomplete: there is no transition to `processed=true`, no `is_approved` flag handling, and no dead-letter audit trail beyond a Kafka send. Ensure state is stored in Postgres consistently with retry outcomes.

## Observability & testing
- No metrics, tracing, or structured error wrapping; add at least request logging, correlation IDs, and Kafka/Postgres operation metrics.
- No tests. Add unit tests for service logic (happy path, retries, DLQ), repository tests with a test DB, and handler tests.

## Operational concerns
- Add health/readiness endpoints for k8s/probes.
- Document how to run the service locally (env vars, Docker compose for Kafka/Postgres, seeding hierarchy.json) and how to bootstrap the database.

Addressing the above will bring the service in line with the intended pipeline and make it production-ready.
