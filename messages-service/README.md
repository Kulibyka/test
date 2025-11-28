# Messages Service

Документация описывает текущее состояние сервиса обработки писем, его API, зависимости и способы запуска.

## Назначение и поток данных
Сервис принимает входящие письма через HTTP, сохраняет их в PostgreSQL и отправляет задачи в Kafka для дальнейшей обработки LLM. Результаты работы модели валидируются и либо сохраняются и публикуются в основной топик, либо при превышении лимита попыток отправляются в dead-letter-топик. Дополнительно предусмотрены ручные операции операторов: получение списка обработанных писем, подтверждение результата и добавление собственного ответа ассистента.

## Архитектура
- **Точка входа** (`cmd/main.go`): инициализирует конфигурацию, логирование, подключения к PostgreSQL и Kafka, создаёт экземпляры сервиса и HTTP-обработчика и запускает HTTP-сервер с graceful shutdown.
- **Бизнес-логика** (`internal/messages`): управляет валидацией входящих данных, подсчётом попыток, отправкой задач в Kafka, обработкой ответов LLM, dead-letter логикой и ручными операциями (аппрув, ответ ассистента, список обработанных писем). При старте сервис также загружает оргструктуру из `configs/hierarchy.json`, если файл доступен.
- **HTTP-транспорт** (`internal/transport/http/messages`): регистрирует REST-эндпоинты и отвечает JSON-структурами с кодами статусов.
- **Хранилище** (`internal/storage`): репозиторий над PostgreSQL со схемой `mails` (см. миграцию `migrations/001_init.sql`).
- **Kafka** (`internal/kafka`): синхронный продюсер на базе `segmentio/kafka-go` с настраиваемыми `acks` и таймаутом.

## Конфигурация
Загрузка происходит через `CONFIG_PATH` (по умолчанию `./configs/messages-service.yaml`). Основные секции файла:
- `env`: `local`/`dev`/`prod` для выбора формата логов.
- `http_server`: адрес, таймаут чтения/записи и idle-таймаут.
- `kafka`: список брокеров и названия топиков (`input_topic`, `output_topic`, `dead_letter_topic`) плюс настройки продюсера (`acks`, `timeout`).
- `retries`: `max_llm_attempts` — лимит неуспешных попыток валидации ответа LLM до помещения сообщения в DLQ.
- `postgresql`: параметры подключения к базе.
- `org`: путь к файлу оргструктуры, загружается best-effort.

Пример валидного файла уже находится в `configs/messages-service.yaml`.

## База данных
Миграция `migrations/001_init.sql` создаёт таблицу `mails` со следующими ключевыми полями:
- `id` (UUID, PK), `input`, `from_email`, `to_email`, `received_at`.
- `attempts`, `status`, флаги `processed`, `is_approved`, `failed_reason`.
- Результаты: `classification`, `model_answer`, `assistant_response`.
- `created_at`/`updated_at` с индексами по `processed`, `status`, `received_at`.

## HTTP API
Все ответы возвращают JSON с полем `error` при ошибках.
- `POST /process` — принимает `id` (опционально), `input`, `from`, `to`, `received_at` (опц.). Сохраняет письмо и публикует задачу в `input_topic`. Ответ: `{"status":"queued","id":"<uuid>"}` со статусом `202`.
- `POST /validate_processed_message` — тело `{id, classification, model_answer}`. При успехе сохраняет результат, публикует его в `output_topic` и отвечает `{"status":"accepted"}`.
- `GET /processed` — возвращает `{"messages":[...]}` со списком обработанных писем из базы.
- `POST /approve` — тело `{id}`. Ставит флаг `is_approved` и отвечает `{"status":"approved","id":"..."}`.
- `POST /add-assistant-response` — тело `{id, assistant_response, mark_processed}`; сохраняет ответ ассистента и опционально помечает письмо обработанным, ответ `{"status":"saved","id":"..."}`.

## Kafka сообщения
- Вход в LLM (`input_topic`): `{"id","input","from","to","received_at"}`.
- Результаты (`output_topic`): `{"id","classification","model_answer"}`.
- Dead-letter (`dead_letter_topic`): `{"id","reason","timestamp","payload"}` где `payload` содержит исходный ответ LLM (если сериализация прошла).

## Запуск локально
1. Требования: Go (go.mod указывает Go 1.25), PostgreSQL, Kafka.
2. Примените миграцию `migrations/001_init.sql` к целевой базе.
3. Заполните `configs/messages-service.yaml` под своё окружение или укажите `CONFIG_PATH` на альтернативный файл.
4. Запустите сервис из корня репозитория: `go run ./messages-service/cmd`.

Логи пишутся в stdout: в текстовом виде для `env=local`, в JSON — для `dev` и `prod`. Остановка по SIGINT/SIGTERM выполняет graceful shutdown HTTP-сервера и закрывает подключения к БД и Kafka.
