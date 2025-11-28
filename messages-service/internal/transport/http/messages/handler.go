package messageshttp

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"backend/messages-service/internal/messages"
)

type Handler struct {
	svc *messages.Service
	log *slog.Logger
}

func New(svc *messages.Service, log *slog.Logger) *Handler {
	return &Handler{
		svc: svc,
		log: log,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/process", h.handleProcess)
	mux.HandleFunc("/validate_processed_message", h.handleValidateProcessedMessage)
}

func (h *Handler) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var dto messages.IncomingMessageDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		h.log.Error("failed to decode /process body", slog.Any("error", err))
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if err := h.svc.ProcessIncomingMessage(r.Context(), dto); err != nil {
		h.log.Error("failed to process incoming message",
			slog.Any("error", err),
		)
		http.Error(w, "failed to process message", http.StatusInternalServerError)
		return
	}

	// На данном этапе нам достаточно статуса, без тела.
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) handleValidateProcessedMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var dto messages.ValidateMessageDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		h.log.Error("failed to decode /validate_processed_message body", slog.Any("error", err))
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if err := h.svc.ValidateProcessedMessage(r.Context(), dto); err != nil {
		h.log.Error("failed to validate processed message",
			slog.Any("error", err),
			slog.String("id", dto.ID),
		)
		http.Error(w, "failed to validate processed message", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
