package messages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Repository описывает, что мы ожидаем от слоя хранения (Postgres и т.п.).
// Конкретная реализация будет в internal/storage (Repo).
type Repository interface {
	CreateMail(ctx context.Context, m *Mail) error
	GetMail(ctx context.Context, id string) (*Mail, error)
	IncrementAttempts(ctx context.Context, id string) error
	MarkAsFailed(ctx context.Context, id string, reason string) error
	SaveLLMResult(ctx context.Context, id string, classification string, modelAnswer json.RawMessage) error
}

// Producer — интерфейс для Kafka-продюсера.
// Реализация будет в internal/kafka.
type Producer interface {
	// Send отправляет сообщение в указанный топик.
	Send(ctx context.Context, topic string, key string, value []byte) error
}

// Mail — доменная сущность письма.
// Хранится в БД.
type Mail struct {
	ID             string          // UUID
	Input          string          // текст письма
	From           string          // from_email
	To             string          // to_email
	ReceivedAt     time.Time       // received_at
	Attempts       int             // attempts
	Status         string          // new / processed / failed / error ...
	Classification string          // класс письма (important/normal/...)
	ModelAnswer    json.RawMessage // сырой json с ответом модели
	FailedReason   string          // причина фейла, если статус failed
}

// DTO, приходящий в /process
type IncomingMessageDTO struct {
	ID         string    `json:"id,omitempty"`
	Input      string    `json:"input"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	ReceivedAt time.Time `json:"received_at,omitempty"`
}

// DTO, приходящий в /validate_processed_message от llm-service
type ValidateMessageDTO struct {
	ID             string          `json:"id"`
	Classification string          `json:"classification"`
	ModelAnswer    json.RawMessage `json:"model_answer"`
}

// Структура задачи, которая уходит в Kafka для llm-service
type LLMTaskMessage struct {
	ID         string    `json:"id"`
	Input      string    `json:"input"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	ReceivedAt time.Time `json:"received_at"`
}

// Структура успешного результата, который уходит в output_topic
type ProcessedMessage struct {
	ID             string          `json:"id"`
	Classification string          `json:"classification"`
	ModelAnswer    json.RawMessage `json:"model_answer"`
}

// Структура, которая уходит в dead_letter_topic
type FailedMessage struct {
	ID        string          `json:"id"`
	Reason    string          `json:"reason"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// Service инкапсулирует бизнес-логику messages-service.
type Service struct {
	repo            Repository
	producer        Producer
	log             *slog.Logger
	maxAttempts     int
	inputTopic      string
	outputTopic     string
	deadLetterTopic string
}

// NewService конструирует Service.
// maxAttempts — лимит попыток обработки LLM.
// inputTopic, outputTopic, deadLetterTopic — названия Kafka-топиков.
func NewService(
	repo Repository,
	producer Producer,
	log *slog.Logger,
	maxAttempts int,
	inputTopic, outputTopic, deadLetterTopic string,
) *Service {
	return &Service{
		repo:            repo,
		producer:        producer,
		log:             log,
		maxAttempts:     maxAttempts,
		inputTopic:      inputTopic,
		outputTopic:     outputTopic,
		deadLetterTopic: deadLetterTopic,
	}
}

// ProcessIncomingMessage — бизнес-логика для эндпоинта /process.
//
// 1. Генерируем ID, если не пришёл.
// 2. Сохраняем письмо в БД (attempts=0, status="new").
// 3. Отправляем задачу в Kafka (inputTopic) для llm-service.
func (s *Service) ProcessIncomingMessage(ctx context.Context, dto IncomingMessageDTO) error {
	if dto.Input == "" {
		return errors.New("input is empty")
	}
	if dto.From == "" || dto.To == "" {
		return errors.New("from/to must be set")
	}

	id := dto.ID
	if id == "" {
		id = uuid.NewString()
	}

	receivedAt := dto.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}

	mail := &Mail{
		ID:         id,
		Input:      dto.Input,
		From:       dto.From,
		To:         dto.To,
		ReceivedAt: receivedAt,
		Attempts:   0,
		Status:     "new",
	}

	if err := s.repo.CreateMail(ctx, mail); err != nil {
		s.log.Error("failed to save mail",
			slog.Any("error", err),
			slog.String("id", id),
		)
		return fmt.Errorf("save mail: %w", err)
	}

	task := LLMTaskMessage{
		ID:         id,
		Input:      mail.Input,
		From:       mail.From,
		To:         mail.To,
		ReceivedAt: mail.ReceivedAt,
	}

	data, err := json.Marshal(task)
	if err != nil {
		s.log.Error("failed to marshal llm task",
			slog.Any("error", err),
			slog.String("id", id),
		)
		return fmt.Errorf("marshal llm task: %w", err)
	}

	if err := s.producer.Send(ctx, s.inputTopic, id, data); err != nil {
		s.log.Error("failed to send llm task to kafka",
			slog.Any("error", err),
			slog.String("id", id),
			slog.String("topic", s.inputTopic),
		)
		return fmt.Errorf("send to kafka: %w", err)
	}

	s.log.Info("incoming message queued for llm",
		slog.String("id", id),
		slog.String("topic", s.inputTopic),
	)

	return nil
}

// ValidateProcessedMessage — бизнес-логика для /validate_processed_message.
//
// 1. Валидация структуры ответа от LLM.
// 2. Если не валидно — увеличиваем attempts, решаем: ретрай или DLQ.
// 3. Если валидно — сохраняем результат в БД и отправляем в output_topic.
func (s *Service) ValidateProcessedMessage(ctx context.Context, dto ValidateMessageDTO) error {
	if dto.ID == "" {
		return errors.New("id is empty")
	}

	// Базовая валидация полей, которые мы ожидаем от LLM
	if err := s.validateLLMOutput(dto); err != nil {
		s.log.Warn("llm output validation failed",
			slog.String("id", dto.ID),
			slog.Any("error", err),
		)
		return s.handleInvalidLLMOutput(ctx, dto, err)
	}

	// Если всё ок — сохраняем результат в БД
	if err := s.repo.SaveLLMResult(ctx, dto.ID, dto.Classification, dto.ModelAnswer); err != nil {
		s.log.Error("failed to save llm result",
			slog.Any("error", err),
			slog.String("id", dto.ID),
		)
		return fmt.Errorf("save llm result: %w", err)
	}

	// Отправляем в output_topic
	msg := ProcessedMessage{
		ID:             dto.ID,
		Classification: dto.Classification,
		ModelAnswer:    dto.ModelAnswer,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		s.log.Error("failed to marshal processed message",
			slog.Any("error", err),
			slog.String("id", dto.ID),
		)
		return fmt.Errorf("marshal processed message: %w", err)
	}

	if err := s.producer.Send(ctx, s.outputTopic, dto.ID, data); err != nil {
		s.log.Error("failed to send processed message to kafka",
			slog.Any("error", err),
			slog.String("id", dto.ID),
			slog.String("topic", s.outputTopic),
		)
		return fmt.Errorf("send processed to kafka: %w", err)
	}

	s.log.Info("llm result accepted",
		slog.String("id", dto.ID),
		slog.String("classification", dto.Classification),
		slog.String("topic", s.outputTopic),
	)

	return nil
}

// validateLLMOutput — базовая валидация JSON от LLM.
// Здесь позже можно прикрутить jsonschema/строгую модель.
func (s *Service) validateLLMOutput(dto ValidateMessageDTO) error {
	if dto.Classification == "" {
		return errors.New("empty classification")
	}
	if len(dto.ModelAnswer) == 0 || string(dto.ModelAnswer) == "null" {
		return errors.New("empty model_answer")
	}

	// сюда можно добавить: распарсить ModelAnswer в конкретный struct и проверить поля

	return nil
}

// handleInvalidLLMOutput — логика при невалидном ответе от LLM.
//
// 1. Получаем письмо из БД, чтобы знать attempts и исходные данные.
// 2. Если attempts+1 >= maxAttempts → шлём в DLQ и помечаем как failed.
// 3. Иначе → attempts++, переотправляем задачу в inputTopic.
func (s *Service) handleInvalidLLMOutput(ctx context.Context, dto ValidateMessageDTO, validationErr error) error {
	mail, err := s.repo.GetMail(ctx, dto.ID)
	if err != nil {
		s.log.Error("failed to get mail for invalid llm output",
			slog.Any("error", err),
			slog.String("id", dto.ID),
		)
		return fmt.Errorf("get mail: %w", err)
	}

	currentAttempts := mail.Attempts

	if currentAttempts+1 >= s.maxAttempts {
		// Достигли лимита — отправляем в DLQ и помечаем как failed.
		reason := fmt.Sprintf("max attempts reached (%d): %v", s.maxAttempts, validationErr)

		if err := s.repo.MarkAsFailed(ctx, dto.ID, reason); err != nil {
			s.log.Error("failed to mark mail as failed",
				slog.Any("error", err),
				slog.String("id", dto.ID),
			)
			return fmt.Errorf("mark as failed: %w", err)
		}

		failedPayload, _ := json.Marshal(dto) // best-effort; если упадёт — просто nil

		failedMsg := FailedMessage{
			ID:        dto.ID,
			Reason:    reason,
			Timestamp: time.Now().UTC(),
			Payload:   failedPayload,
		}

		data, err := json.Marshal(failedMsg)
		if err != nil {
			s.log.Error("failed to marshal failed message",
				slog.Any("error", err),
				slog.String("id", dto.ID),
			)
			return fmt.Errorf("marshal failed message: %w", err)
		}

		if err := s.producer.Send(ctx, s.deadLetterTopic, dto.ID, data); err != nil {
			s.log.Error("failed to send message to dead_letter_topic",
				slog.Any("error", err),
				slog.String("id", dto.ID),
				slog.String("topic", s.deadLetterTopic),
			)
			return fmt.Errorf("send to dlq: %w", err)
		}

		s.log.Info("message sent to dead_letter_topic",
			slog.String("id", dto.ID),
			slog.Int("attempts", currentAttempts+1),
		)

		return nil
	}

	// Ещё можем пробовать — инкремент attempts и переотправляем задачу в inputTopic
	if err := s.repo.IncrementAttempts(ctx, dto.ID); err != nil {
		s.log.Error("failed to increment attempts",
			slog.Any("error", err),
			slog.String("id", dto.ID),
		)
		return fmt.Errorf("increment attempts: %w", err)
	}

	task := LLMTaskMessage{
		ID:         mail.ID,
		Input:      mail.Input,
		From:       mail.From,
		To:         mail.To,
		ReceivedAt: mail.ReceivedAt,
	}

	data, err := json.Marshal(task)
	if err != nil {
		s.log.Error("failed to marshal llm retry task",
			slog.Any("error", err),
			slog.String("id", dto.ID),
		)
		return fmt.Errorf("marshal llm retry task: %w", err)
	}

	if err := s.producer.Send(ctx, s.inputTopic, dto.ID, data); err != nil {
		s.log.Error("failed to send llm retry task to kafka",
			slog.Any("error", err),
			slog.String("id", dto.ID),
			slog.String("topic", s.inputTopic),
		)
		return fmt.Errorf("send retry to kafka: %w", err)
	}

	s.log.Info("llm task requeued",
		slog.String("id", dto.ID),
		slog.Int("attempts", currentAttempts+1),
		slog.String("topic", s.inputTopic),
	)

	return nil
}
