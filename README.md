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
2. Services expose the following ports on your host:
   - messages-service: `http://localhost:8080`
   - llm-service: `http://localhost:8081`
   - Kafka broker: `localhost:9092`
   - Postgres: `localhost:5432` (database `emails`, user/password `postgres`)

Configuration defaults match the values in `messages-service/configs/messages-service.yaml`. Override the config path by setting `CONFIG_PATH` if needed.
