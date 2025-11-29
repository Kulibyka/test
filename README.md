# Mail processing stack

This repository hosts two Go services and supporting infrastructure used to process incoming mails with an LLM.

- **messages-service** — HTTP API for ingesting mails, persisting them to Postgres and publishing tasks to Kafka.
- **llm-service** — HTTP worker that wraps the LLM call and returns JSON responses.
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

Configuration defaults match the values in `messages-service/configs/messages-service.yaml`. Override the config path by setting `CONFIG_PATH` if needed.
