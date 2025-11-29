# Messages Service

Расширенная документация описывает текущее состояние сервиса обработки писем, доступные компоненты кода, API, данные и инструкции по запуску. Текст отражает фактическое содержимое репозитория на момент последнего коммита.

## Обзор назначения и потока данных
Сервис принимает входящие письма через HTTP, сохраняет их в PostgreSQL и публикует задачи в Kafka для дальнейшей обработки LLM. Ответы модели валидируются: корректные результаты пишутся в базу и отправляются в основной топик, а превышение лимита попыток отправляет событие в dead-letter-топик. Операторы могут просматривать обработанные письма, подтверждать результаты и прикладывать собственные ответы ассистента.

## Структура проекта и описание файлов
### Корень
- `cmd/main.go` — точка входа приложения: загружает конфигурацию, настраивает логирование (`internal/logger`), инициализирует Postgres (через `internal/storage/postgresql`, отсутствует в репозитории), создаёт репозиторий (`internal/storage`), Kafka-продюсер (`internal/kafka`), сервис (`internal/messages`) и HTTP-обработчики (`internal/transport/http/messages`). Реализован graceful shutdown HTTP-сервера и закрытие ресурсов.
- `README.md` — текущая документация (этот файл).
- `TODO.md` — перечень известных разрывов между реализацией и желаемым поведением: несоответствие конфигурации, пробелы в HTTP-ручках, проверках, логировании, миграциях, потребителях Kafka и тестах.
- `configs/messages-service.yaml` — пример файла настроек, соответствующий структуре `internal/config/config.go`.
- `configs/hierarchy.json` — пример оргструктуры; сервис подгружает её best-effort при старте и пишет в лог количество узлов.
- `migrations/001_init.sql` — единственная миграция, создаёт таблицу `mails` и индексы по `processed`, `status`, `received_at`.

### Конфигурация (`internal/config/config.go`)
- Структура `Config` содержит секции `env`, `http_server`, `kafka`, `retries`, `postgresql`, `org`.
- `MustLoad()` читает путь из `CONFIG_PATH` (по умолчанию `./configs/messages-service.yaml`), проверяет наличие файла и парсит YAML через `cleanenv`. Без файла процесс аварийно завершается через `log.Fatalf`.
- Конфигурация HTTP: адрес, таймауты чтения/записи и idle.
- Kafka: брокеры, топики для входа/выхода/DLQ и настройки продюсера (`acks`, `timeout`).
- Retries: максимальное число неудачных попыток валидации LLM (`max_llm_attempts`).
- PostgreSQL: параметры подключения (`host`, `port`, `user`, `password`, `dbname`, `sslmode`).
- Org: путь к `hierarchy.json`.

### Логирование (`internal/logger/logger.go`)
- `New(env string)` возвращает `slog.Logger` с разными форматами и уровнями:
  - `local` — текст, уровень DEBUG;
  - `dev` — JSON, DEBUG;
  - `prod` — JSON, INFO;
  - значение по умолчанию — текст, INFO.

### Бизнес-логика (`internal/messages/service.go`)
- Интерфейсы `Repository` и `Producer` описывают зависимости от хранилища и Kafka.
- `Mail` — доменная модель с полями БД (включая `Attempts`, `Status`, `Classification`, JSON-ответы модели и ассистента, флаги `Processed`/`IsApproved`, `FailedReason`, `UpdatedAt`).
- DTO:
  - `IncomingMessageDTO` (`/process`),
  - `ValidateMessageDTO` (LLM → `/validate_processed_message`),
  - `AssistantResponseDTO` (`/add-assistant-response`),
  - `ApproveDTO` (`/approve`).
- Сообщения Kafka:
  - `LLMTaskMessage` (отправляется в `input_topic`),
  - `ProcessedMessage` (в `output_topic` после успешной валидации),
  - `FailedMessage` (в `dead_letter_topic` при превышении лимитов).
- `NewService` загружает оргструктуру из `hierarchyPath` (если файл доступен) и настраивает лимит попыток.
- `ProcessIncomingMessage` валидирует адреса и тело письма, генерирует `id` при отсутствии, сохраняет запись с `status="new"`, публикует задачу в Kafka.
- `ValidateProcessedMessage` проверяет наличие `id`, валидирует JSON ответа LLM, либо сохраняет результат + шлёт в `output_topic`, либо вызывает `handleInvalidLLMOutput`.
- `validateLLMOutput` — базовые проверки непустых полей и корректности JSON.
- `handleInvalidLLMOutput` — читает письмо из БД, при достижении лимита `maxAttempts` помечает запись как `failed` и отправляет событие в DLQ; иначе инкрементирует `attempts` и переотправляет задачу в `input_topic`.
- `GetProcessedMessages` — возвращает обработанные письма.
- `ApproveMessage` — ставит флаг подтверждения.
- `AddAssistantResponse` — валидирует JSON ответа ассистента и сохраняет его, опционально помечая письмо обработанным.

### Хранилище (`internal/storage/repo.go`)
- `Repo` работает поверх `*sql.DB` и реализует интерфейс `messages.Repository`.
- Операции:
  - `CreateMail` — вставка новой строки с начальными полями и флагами;
  - `GetMail` — выборка одной записи c приведением nullable полей;
  - `IncrementAttempts` — `attempts++` и обновление `updated_at`;
  - `SaveLLMResult` — запись классификации, ответа модели, сброс attempts, выставление `processed=true` и `status='processed'`;
  - `MarkAsFailed` — установка статуса `failed`, причины и `processed=false`;
  - `ListProcessed` — выборка обработанных писем (`processed=true`) в порядке `updated_at DESC`;
  - `ApproveMail` — проставление `is_approved=true`;
  - `SaveAssistantResponse` — сохранение ответа ассистента и, при необходимости, установка `processed=true` и `status='processed'`.

### Kafka (`internal/kafka/producer.go`)
- `Producer` строит синхронный `kafka.Writer` (`segmentio/kafka-go`), задаёт балансировщик `LeastBytes`, `RequiredAcks` через `parseAcks`, опционально ограничивает таймаут отправки.
- `Send` формирует `kafka.Message` с ключом/значением и текущим временем, оборачивает отправку в контекст с таймаутом, логирует ошибки и DEBUG-сообщение об успешной отправке.
- `Close` закрывает writer с предупреждением при ошибке.

### HTTP-слой (`internal/transport/http/messages/handler.go`)
- Регистрация маршрутов: `/process`, `/validate_processed_message`, `/processed`, `/approve`, `/add-assistant-response` на `http.ServeMux`.
- Каждый handler проверяет HTTP-метод, парсит JSON, вызывает соответствующий метод сервиса и отвечает структурой JSON (успех/ошибка) с кодом статуса.

### Данные и миграции
- Таблица `mails` (см. `migrations/001_init.sql`): хранит входные письма, попытки, статус, классификацию, ответы модели и ассистента, флаги обработанности и утверждения, таймстампы и индексы для запросов по состоянию.

## HTTP API (фактические обработчики)
Все ответы — JSON, при ошибке возвращается поле `error`.
- `POST /process` — тело `{id?, input, from, to, received_at?}`; сохраняет письмо и публикует задачу в `input_topic`. Ответ `202` `{status:"queued", id}`.
- `POST /validate_processed_message` — тело `{id, classification, model_answer}`; при успехе сохраняет результат, отправляет в `output_topic`, ответ `200` `{status:"accepted"}`. При невалидных данных — ретраи или DLQ по логике сервиса.
- `GET /processed` — возвращает `200` `{messages:[...]}` со списком обработанных писем.
- `POST /approve` — тело `{id}`; устанавливает `is_approved=true`, ответ `200` `{status:"approved", id}`.
- `POST /add-assistant-response` — тело `{id, assistant_response, mark_processed}`; сохраняет ответ ассистента и опционально помечает письмо обработанным, ответ `200` `{status:"saved", id}`.

## Kafka сообщения
- Вход в LLM (`input_topic`): `{id, input, from, to, received_at}`.
- Результаты (`output_topic`): `{id, classification, model_answer}`.
- Dead-letter (`dead_letter_topic`): `{id, reason, timestamp, payload}` (payload — исходный ответ LLM, если сериализация прошла).

## Запуск локально
1. Требования: Go (go.mod указывает Go 1.25), PostgreSQL, Kafka.
2. Примените миграцию `migrations/001_init.sql` к базе `emails` или другой, соответствующей `postgresql` в конфиге.
3. Заполните `configs/messages-service.yaml` или задайте `CONFIG_PATH` на альтернативный путь; убедитесь, что брокеры Kafka и БД доступны.
4. Запустите сервис из корня репозитория: `go run ./messages-service/cmd`.
5. Логи пишутся в stdout: текст для `env=local`, JSON для `env=dev/prod`.
6. Завершение по SIGINT/SIGTERM инициирует graceful shutdown HTTP-сервера и закрытие подключений к БД и Kafka.

## Известные ограничения (см. TODO.md)
- В репозитории отсутствует реализация клиента Postgres (`internal/storage/postgresql`) и потребители Kafka; тесты не написаны.
- Валидация входящих данных базовая; нет схем для проверки структур ответов LLM и ассистента.
- Нет метрик/трассировки, health/readiness эндпоинтов и автоматизации запуска зависимостей.
Footer
© 2025 GitHub, Inc.
Footer navigation
Terms
Privacy
Security
