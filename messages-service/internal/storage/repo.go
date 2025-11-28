package storage

import (
	"backend/messages-service/internal/messages"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// Repo реализует интерфейс messages.Repository
type Repo struct {
	db *sql.DB
}

func NewMessagesRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) CreateMail(ctx context.Context, m *messages.Mail) error {
	const query = `
		INSERT INTO mails 
			(id, input, from_email, to_email, received_at, attempts, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7);
	`

	_, err := r.db.ExecContext(ctx, query,
		m.ID,
		m.Input,
		m.From,
		m.To,
		m.ReceivedAt,
		m.Attempts,
		m.Status,
	)
	return err
}

func (r *Repo) GetMail(ctx context.Context, id string) (*messages.Mail, error) {
	const query = `
		SELECT 
			id,
			input,
			from_email,
			to_email,
			received_at,
			attempts,
			status,
			classification,
			model_answer,
			failed_reason
		FROM mails
		WHERE id = $1;
	`

	row := r.db.QueryRowContext(ctx, query, id)

	var mail messages.Mail
	var modelAnswer json.RawMessage
	var classification sql.NullString
	var failedReason sql.NullString

	err := row.Scan(
		&mail.ID,
		&mail.Input,
		&mail.From,
		&mail.To,
		&mail.ReceivedAt,
		&mail.Attempts,
		&mail.Status,
		&classification,
		&modelAnswer,
		&failedReason,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("mail not found: %w", err)
		}
		return nil, err
	}

	if classification.Valid {
		mail.Classification = classification.String
	}
	if modelAnswer != nil {
		mail.ModelAnswer = modelAnswer
	}
	if failedReason.Valid {
		mail.FailedReason = failedReason.String
	}

	return &mail, nil
}

func (r *Repo) IncrementAttempts(ctx context.Context, id string) error {
	const query = `
		UPDATE mails
		SET attempts = attempts + 1,
		    updated_at = NOW()
		WHERE id = $1;
	`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("mail id %s not found", id)
	}

	return nil
}

func (r *Repo) SaveLLMResult(ctx context.Context, id string, classification string, modelAnswer json.RawMessage) error {
	const query = `
		UPDATE mails
		SET classification = $2,
		    model_answer = $3,
		    status = 'processed',
		    updated_at = NOW()
		WHERE id = $1;
	`

	res, err := r.db.ExecContext(ctx, query,
		id,
		classification,
		modelAnswer,
	)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("mail id %s not found", id)
	}

	return nil
}

func (r *Repo) MarkAsFailed(ctx context.Context, id string, reason string) error {
	const query = `
		UPDATE mails
		SET status = 'failed',
		    failed_reason = $2,
		    updated_at = NOW()
		WHERE id = $1;
	`

	res, err := r.db.ExecContext(ctx, query,
		id,
		reason,
	)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("mail id %s not found", id)
	}

	return nil
}
