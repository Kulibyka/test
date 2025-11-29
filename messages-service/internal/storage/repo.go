package storage

import (
	"backend/messages-service/internal/messages"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type Repo struct {
	db *sql.DB
}

func NewMessagesRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) CreateMail(ctx context.Context, m *messages.Mail) error {
	const query = `
INSERT INTO mails
(id, input, from_email, to_email, received_at, attempts, status, processed, is_approved)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
`

	_, err := r.db.ExecContext(ctx, query,
		m.ID,
		m.Input,
		m.From,
		m.To,
		m.ReceivedAt,
		m.Attempts,
		m.Status,
		m.Processed,
		m.IsApproved,
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
failed_reason,
assistant_response,
processed,
is_approved,
updated_at
FROM mails
WHERE id = $1;
`

	row := r.db.QueryRowContext(ctx, query, id)

	var mail messages.Mail
	var modelAnswer json.RawMessage
	var assistantResponse sql.NullString
	var classification sql.NullString
	var failedReason sql.NullString
	var processed sql.NullBool
	var approved sql.NullBool

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
		&assistantResponse,
		&processed,
		&approved,
		&mail.UpdatedAt,
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
	if assistantResponse.Valid {
		mail.AssistantResp = json.RawMessage(assistantResponse.String)
	}
	if failedReason.Valid {
		mail.FailedReason = failedReason.String
	}
	if processed.Valid {
		mail.Processed = processed.Bool
	}
	if approved.Valid {
		mail.IsApproved = approved.Bool
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
processed = TRUE,
status = 'processed',
attempts = 0,
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
processed = FALSE,
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

func (r *Repo) ListProcessed(ctx context.Context) ([]messages.Mail, error) {
	const query = `
SELECT id, input, from_email, to_email, received_at, attempts, status, classification, model_answer, assistant_response, is_approved, updated_at
FROM mails
WHERE processed = TRUE
ORDER BY updated_at DESC;
`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mails []messages.Mail
	for rows.Next() {
		var mail messages.Mail
		var assistantResponse sql.NullString
		if err := rows.Scan(
			&mail.ID,
			&mail.Input,
			&mail.From,
			&mail.To,
			&mail.ReceivedAt,
			&mail.Attempts,
			&mail.Status,
			&mail.Classification,
			&mail.ModelAnswer,
			&assistantResponse,
			&mail.IsApproved,
			&mail.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if assistantResponse.Valid {
			mail.AssistantResp = json.RawMessage(assistantResponse.String)
		}
		mail.Processed = true
		mails = append(mails, mail)
	}

	return mails, nil
}

func (r *Repo) ApproveMail(ctx context.Context, id string) error {
	const query = `
UPDATE mails
SET is_approved = TRUE,
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

func (r *Repo) SaveAssistantResponse(ctx context.Context, id string, response json.RawMessage, markProcessed bool) error {
	const query = `
UPDATE mails
SET assistant_response = $2,
processed = CASE WHEN $3 THEN TRUE ELSE processed END,
status = CASE WHEN $3 THEN 'processed' ELSE status END,
updated_at = NOW()
WHERE id = $1;
`

	res, err := r.db.ExecContext(ctx, query, id, response, markProcessed)
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
