# Mail processing stack

This repository hosts two Go services and supporting infrastructure used to process incoming mails with an LLM.

- **messages-service** — HTTP API for ingesting mails, persisting them to Postgres and publishing tasks to Kafka.
- **llm-service** — HTTP proxy around OpenRouter: forwards the mail body to a hosted LLM and returns the extracted JSON answer. Requires `OPENROUTER_API_KEY`.
- **docker-compose** — Local runtime for Postgres, Kafka/ZooKeeper and both services.

## Running locally with Docker Compose
1. Build and start the stack:
   ```bash
   docker compose up --build
   ```
2. Apply the database schema for `messages-service` (the migration lives in `messages-service/migrations`):
   - If you have `psql` installed locally:
     ```bash
     psql postgres://postgres:postgres@localhost:5432/emails -f messages-service/migrations/001_init.up.sql
     ```
   - Or run the migration through the running Postgres container (no local `psql` needed):
     ```bash
     docker compose exec -T postgres psql -U postgres -d emails < messages-service/migrations/001_init.up.sql
     ```
   - Running the migration *from* the `messages-service` container is not recommended: the runtime image is
     distroless (no shell, no `psql` client), so it is not suitable for applying SQL. Use the Postgres
     container or a separate tooling image instead.
3. Services expose the following ports on your host:
   - messages-service: `http://localhost:8080`
   - llm-service: `http://localhost:8081`
   - Kafka broker: `localhost:9092`
   - Postgres: `localhost:5432` (database `emails`, user/password `postgres`)

Configuration defaults match the values in `messages-service/configs/messages-service.yaml`. Override the config path by setting `CONFIG_PATH` if needed. `llm-service` listens on `PORT` (default `8080`) and needs `OPENROUTER_API_KEY` in the environment (export it before running `docker compose up`).

### Useful endpoints

- `GET /healthz` — health probes for both services.
- `POST /process` — submit incoming mail to `messages-service` (JSON body: `input`, `from`, `to`, optional `id`). The service persists the message and enqueues it to Kafka.
- `POST /validate_processed_message` — accept LLM results for a message.
- `GET /processed` — list processed messages.
- `POST /approve` and `POST /add-assistant-response` — operator actions.
- `POST /process` on `llm-service` — forwards the raw request body to the configured OpenRouter model (default `openai/gpt-4o`), extracts JSON from the response, validates it, and returns it to the caller.
