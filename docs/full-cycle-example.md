# Пример полного цикла (Postman)

Ниже — один связный сценарий из семи запросов, который можно воспроизвести в Postman (или curl) после запуска `docker compose up --build`. Все URL приведены для локального запуска: `messages-service` — `http://localhost:8080`, `llm-service` — `http://localhost:8081`.

## 1. Проверка здоровья сервисов
- **Запрос:** `GET http://localhost:8080/healthz` и `GET http://localhost:8081/healthz`
- **Ожидание:** статус `200` и тело `{ "status": "ok" }` (messages-service) / `ok` (llm-service).

## 2. Поставить письмо в очередь
- **Запрос:** `POST http://localhost:8080/process`
- **Тело (raw JSON):**
```json
{
  "input": "Hi, I'd like to schedule a demo next week.",
  "from": "sender@example.com",
  "to": "support@example.com"
}
```
- **Ответ (пример):**
```json
{
  "status": "queued",
  "id": "9c8f3b5c-7c02-4c94-9f6d-2f5c8d0b3b77"
}
```
- **Дальше:** сохраните `id` из ответа.

## 3. Отправить сырой текст в llm-service (опционально)
- **Запрос:** `POST http://localhost:8081/process`
- **Тело (raw text):**
```
{"id":"9c8f3b5c-7c02-4c94-9f6d-2f5c8d0b3b77","input":"Hi, I'd like to schedule a demo next week."}
```
- **Ответ (пример):**
```json
{"summary":"User wants a product demo next week","priority":"high"}
```
- **Дальше:** этот JSON надо передать в шаге 4 как `model_answer`.

## 4. Сохранить классификацию и ответ модели
- **Запрос:** `POST http://localhost:8080/validate_processed_message`
- **Тело (raw JSON):**
```json
{
  "id": "9c8f3b5c-7c02-4c94-9f6d-2f5c8d0b3b77",
  "classification": "important",
  "model_answer": {"summary":"User wants a product demo next week","priority":"high"}
}
```
- **Ответ (пример):** `{ "status": "accepted" }`.

## 5. Проверить, что данные сохранены
- **Запрос:** `GET http://localhost:8080/processed`
- **Ожидание:** в массиве `messages` найдите объект с вашим `id`; поля `classification` и `model_answer` должны совпадать с шагом 4.

## 6. Одобрить сообщение
- **Запрос:** `POST http://localhost:8080/approve`
- **Тело (raw JSON):**
```json
{
  "id": "9c8f3b5c-7c02-4c94-9f6d-2f5c8d0b3b77"
}
```
- **Ответ (пример):** `{ "status": "approved", "id": "9c8f3b5c-7c02-4c94-9f6d-2f5c8d0b3b77" }`.

## 7. (Опционально) Добавить или заменить ответ ассистента
- **Запрос:** `POST http://localhost:8080/add-assistant-response`
- **Тело (raw JSON):**
```json
{
  "id": "9c8f3b5c-7c02-4c94-9f6d-2f5c8d0b3b77",
  "assistant_response": {"summary": "Confirmed demo for next Tuesday at 11:00"},
  "mark_processed": true
}
```
- **Ответ (пример):** `{ "status": "saved", "id": "9c8f3b5c-7c02-4c94-9f6d-2f5c8d0b3b77" }`.

После шага 6 или 7 сообщение считается обработанным и готовым для дальнейших систем.
