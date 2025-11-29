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
	mux.HandleFunc("/processed", h.handleGetProcessed)
	mux.HandleFunc("/approve", h.handleApprove)
	mux.HandleFunc("/add-assistant-response", h.handleAddAssistantResponse)
	mux.HandleFunc("/healthz", h.handleHealth)
}

func (h *Handler) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	defer r.Body.Close()

	var dto messages.IncomingMessageDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		h.log.Error("failed to decode /process body", slog.Any("error", err))
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	id, err := h.svc.ProcessIncomingMessage(r.Context(), dto)
	if err != nil {
		h.log.Error("failed to process incoming message",
			slog.Any("error", err),
		)
		writeError(w, http.StatusInternalServerError, "failed to process message")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "id": id})
}

func (h *Handler) handleValidateProcessedMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	defer r.Body.Close()

	var dto messages.ValidateMessageDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		h.log.Error("failed to decode /validate_processed_message body", slog.Any("error", err))
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if err := h.svc.ValidateProcessedMessage(r.Context(), dto); err != nil {
		h.log.Error("failed to validate processed message",
			slog.Any("error", err),
			slog.String("id", dto.ID),
		)
		writeError(w, http.StatusInternalServerError, "failed to validate processed message")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

func (h *Handler) handleGetProcessed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	items, err := h.svc.GetProcessedMessages(r.Context())
	if err != nil {
		h.log.Error("failed to list processed messages", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to list processed messages")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"messages": items})
}

func (h *Handler) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	defer r.Body.Close()
	var dto messages.ApproveDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		h.log.Error("failed to decode approve body", slog.Any("error", err))
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if err := h.svc.ApproveMessage(r.Context(), dto); err != nil {
		h.log.Error("failed to approve message", slog.Any("error", err), slog.String("id", dto.ID))
		writeError(w, http.StatusInternalServerError, "failed to approve message")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "approved", "id": dto.ID})
}

func (h *Handler) handleAddAssistantResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	defer r.Body.Close()
	var dto messages.AssistantResponseDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		h.log.Error("failed to decode add assistant response body", slog.Any("error", err))
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if err := h.svc.AddAssistantResponse(r.Context(), dto); err != nil {
		h.log.Error("failed to save assistant response", slog.Any("error", err), slog.String("id", dto.ID))
		writeError(w, http.StatusInternalServerError, "failed to save assistant response")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved", "id": dto.ID})
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
