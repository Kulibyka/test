package messages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"os"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	CreateMail(ctx context.Context, m *Mail) error
	GetMail(ctx context.Context, id string) (*Mail, error)
	IncrementAttempts(ctx context.Context, id string) error
	MarkAsFailed(ctx context.Context, id string, reason string) error
	SaveLLMResult(ctx context.Context, id string, classification string, modelAnswer json.RawMessage) error
	ListProcessed(ctx context.Context) ([]Mail, error)
	ApproveMail(ctx context.Context, id string) error
	SaveAssistantResponse(ctx context.Context, id string, response json.RawMessage, markProcessed bool) error
}

type Producer interface {
	Send(ctx context.Context, topic string, key string, value []byte) error
}

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
	AssistantResp  json.RawMessage // ответ ассистента, если он добавлен вручную
	Processed      bool            // processed flag
	IsApproved     bool            // оператор утвердил ответ
	FailedReason   string          // причина фейла, если статус failed
	UpdatedAt      time.Time       // updated_at
}

type IncomingMessageDTO struct {
	ID         string    `json:"id,omitempty"`
	Input      string    `json:"input"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	ReceivedAt time.Time `json:"received_at,omitempty"`
}

type ValidateMessageDTO struct {
	ID             string          `json:"id"`
	Classification string          `json:"classification"`
	ModelAnswer    json.RawMessage `json:"model_answer"`
}

type AssistantResponseDTO struct {
	ID                string          `json:"id"`
	AssistantResponse json.RawMessage `json:"assistant_response"`
	MarkProcessed     bool            `json:"mark_processed"`
}

type ApproveDTO struct {
	ID string `json:"id"`
}

type LLMTaskMessage struct {
	ID         string    `json:"id"`
	Input      string    `json:"input"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	ReceivedAt time.Time `json:"received_at"`
}

type ProcessedMessage struct {
	ID             string          `json:"id"`
	Classification string          `json:"classification"`
	ModelAnswer    json.RawMessage `json:"model_answer"`
}

type FailedMessage struct {
	ID        string          `json:"id"`
	Reason    string          `json:"reason"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type Service struct {
	repo            Repository
	producer        Producer
	log             *slog.Logger
	maxAttempts     int
	inputTopic      string
	outputTopic     string
	deadLetterTopic string
	hierarchy       map[string]any
}

func NewService(
	repo Repository,
	producer Producer,
	log *slog.Logger,
	maxAttempts int,
	inputTopic, outputTopic, deadLetterTopic string,
	hierarchyPath string,
) *Service {
	hierarchy := loadHierarchy(hierarchyPath, log)

	return &Service{
		repo:            repo,
		producer:        producer,
		log:             log,
		maxAttempts:     maxAttempts,
		inputTopic:      inputTopic,
		outputTopic:     outputTopic,
		deadLetterTopic: deadLetterTopic,
		hierarchy:       hierarchy,
	}
}

func (s *Service) ProcessIncomingMessage(ctx context.Context, dto IncomingMessageDTO) (string, error) {
	if dto.Input == "" {
		return "", errors.New("input is empty")
	}
	if dto.From == "" || dto.To == "" {
		return "", errors.New("from/to must be set")
	}

	if _, err := mail.ParseAddress(dto.From); err != nil {
		return "", fmt.Errorf("invalid from address: %w", err)
	}
	if _, err := mail.ParseAddress(dto.To); err != nil {
		return "", fmt.Errorf("invalid to address: %w", err)
	}

	id := dto.ID
	if id == "" {
		id = uuid.NewString()
	}

	receivedAt := dto.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}

	mailEntity := &Mail{
		ID:         id,
		Input:      dto.Input,
		From:       dto.From,
		To:         dto.To,
		ReceivedAt: receivedAt,
		Attempts:   0,
		Status:     "new",
		Processed:  false,
		IsApproved: false,
	}

	if err := s.repo.CreateMail(ctx, mailEntity); err != nil {
		s.log.Error("failed to save mail",
			slog.Any("error", err),
			slog.String("id", id),
		)
		return "", fmt.Errorf("save mail: %w", err)
	}

	task := LLMTaskMessage{
		ID:         id,
		Input:      mailEntity.Input,
		From:       mailEntity.From,
		To:         mailEntity.To,
		ReceivedAt: mailEntity.ReceivedAt,
	}

	data, err := json.Marshal(task)
	if err != nil {
		s.log.Error("failed to marshal llm task",
			slog.Any("error", err),
			slog.String("id", id),
		)
		return "", fmt.Errorf("marshal llm task: %w", err)
	}

	if err := s.producer.Send(ctx, s.inputTopic, id, data); err != nil {
		s.log.Error("failed to send llm task to kafka",
			slog.Any("error", err),
			slog.String("id", id),
			slog.String("topic", s.inputTopic),
		)
		return "", fmt.Errorf("send to kafka: %w", err)
	}

	s.log.Info("incoming message queued for llm",
		slog.String("id", id),
		slog.String("topic", s.inputTopic),
	)

	return id, nil
}

func (s *Service) ValidateProcessedMessage(ctx context.Context, dto ValidateMessageDTO) error {
	if dto.ID == "" {
		return errors.New("id is empty")
	}

	if err := s.validateLLMOutput(dto); err != nil {
		s.log.Warn("llm output validation failed",
			slog.String("id", dto.ID),
			slog.Any("error", err),
		)
		return s.handleInvalidLLMOutput(ctx, dto, err)
	}

	if err := s.repo.SaveLLMResult(ctx, dto.ID, dto.Classification, dto.ModelAnswer); err != nil {
		s.log.Error("failed to save llm result",
			slog.Any("error", err),
			slog.String("id", dto.ID),
		)
		return fmt.Errorf("save llm result: %w", err)
	}

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

func (s *Service) validateLLMOutput(dto ValidateMessageDTO) error {
	if dto.Classification == "" {
		return errors.New("empty classification")
	}
	if len(dto.ModelAnswer) == 0 || string(dto.ModelAnswer) == "null" {
		return errors.New("empty model_answer")
	}

	var parsed any
	if err := json.Unmarshal(dto.ModelAnswer, &parsed); err != nil {
		return fmt.Errorf("invalid model_answer json: %w", err)
	}

	return nil
}

func (s *Service) handleInvalidLLMOutput(ctx context.Context, dto ValidateMessageDTO, validationErr error) error {
	mailEntity, err := s.repo.GetMail(ctx, dto.ID)
	if err != nil {
		s.log.Error("failed to get mail for invalid llm output",
			slog.Any("error", err),
			slog.String("id", dto.ID),
		)
		return fmt.Errorf("get mail: %w", err)
	}

	currentAttempts := mailEntity.Attempts

	if currentAttempts+1 >= s.maxAttempts {
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

	if err := s.repo.IncrementAttempts(ctx, dto.ID); err != nil {
		s.log.Error("failed to increment attempts",
			slog.Any("error", err),
			slog.String("id", dto.ID),
		)
		return fmt.Errorf("increment attempts: %w", err)
	}

	task := LLMTaskMessage{
		ID:         mailEntity.ID,
		Input:      mailEntity.Input,
		From:       mailEntity.From,
		To:         mailEntity.To,
		ReceivedAt: mailEntity.ReceivedAt,
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

func (s *Service) GetProcessedMessages(ctx context.Context) ([]Mail, error) {
	mails, err := s.repo.ListProcessed(ctx)
	if err != nil {
		return nil, fmt.Errorf("list processed: %w", err)
	}
	return mails, nil
}

func (s *Service) ApproveMessage(ctx context.Context, dto ApproveDTO) error {
	if dto.ID == "" {
		return errors.New("id is empty")
	}

	if err := s.repo.ApproveMail(ctx, dto.ID); err != nil {
		return fmt.Errorf("approve mail: %w", err)
	}
	return nil
}

func (s *Service) AddAssistantResponse(ctx context.Context, dto AssistantResponseDTO) error {
	if dto.ID == "" {
		return errors.New("id is empty")
	}
	if len(dto.AssistantResponse) == 0 || string(dto.AssistantResponse) == "null" {
		return errors.New("assistant_response is empty")
	}

	var parsed any
	if err := json.Unmarshal(dto.AssistantResponse, &parsed); err != nil {
		return fmt.Errorf("invalid assistant_response json: %w", err)
	}

	if err := s.repo.SaveAssistantResponse(ctx, dto.ID, dto.AssistantResponse, dto.MarkProcessed); err != nil {
		return fmt.Errorf("save assistant response: %w", err)
	}
	return nil
}

func loadHierarchy(path string, log *slog.Logger) map[string]any {
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Warn("failed to read hierarchy file", slog.Any("error", err), slog.String("path", path))
		return nil
	}

	var hierarchy map[string]any
	if err := json.Unmarshal(data, &hierarchy); err != nil {
		log.Warn("failed to parse hierarchy file", slog.Any("error", err), slog.String("path", path))
		return nil
	}

	log.Info("hierarchy loaded", slog.Int("nodes", len(hierarchy)))

	return hierarchy
}
