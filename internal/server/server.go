package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"web-ai/internal/chat"
	"web-ai/internal/config"
	"web-ai/internal/session"
	"web-ai/internal/store"
)

type contextKey string

const userIDKey contextKey = "user_id"

type Server struct {
	cfg      *config.Config
	store    *store.Store
	sessions *session.Manager
	chat     *chat.Service
	static   fs.FS
}

func New(cfg *config.Config, st *store.Store, sessions *session.Manager, chat *chat.Service, staticFS fs.FS) *Server {
	return &Server{cfg: cfg, store: st, sessions: sessions, chat: chat, static: staticFS}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/public/config", s.handlePublicConfig)
	mux.HandleFunc("/api/session/login", s.handleLogin)
	mux.Handle("/api/session/me", s.auth(http.HandlerFunc(s.handleSessionMe)))
	mux.Handle("/api/models", s.auth(http.HandlerFunc(s.handleModels)))
	mux.Handle("/api/conversations", s.auth(http.HandlerFunc(s.handleConversations)))
	mux.Handle("/api/conversations/", s.auth(http.HandlerFunc(s.handleConversationByID)))
	mux.Handle("/api/messages/partial", s.auth(http.HandlerFunc(s.handlePartialAssistantMessage)))
	mux.Handle("/api/messages/", s.auth(http.HandlerFunc(s.handleMessageByID)))
	mux.Handle("/api/chat/completions", s.auth(http.HandlerFunc(s.handleChatCompletions)))
	mux.Handle("/", spaHandler(http.FS(s.static)))
	return mux
}

func (s *Server) handlePublicConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"title": s.cfg.UI.Title,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var request struct {
		UserID string `json:"user_id"`
	}
	if err := decodeJSON(r.Body, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	request.UserID = strings.TrimSpace(request.UserID)
	if request.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if !s.cfg.IsAllowedUser(request.UserID) {
		writeError(w, http.StatusUnauthorized, "user_id is not allowed")
		return
	}
	token, expiresAt, err := s.sessions.Create(request.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"user_id":    request.UserID,
		"expires_at": expiresAt,
	})
}

func (s *Server) handleSessionMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	userID := userIDFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": userID,
		"title":   s.cfg.UI.Title,
	})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"models":        s.cfg.Models,
		"title_model":   s.cfg.TitleModel,
		"default_model": s.cfg.Models[0].ID,
	})
}

func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		conversations, err := s.store.ListConversations(userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"conversations": conversations})
	case http.MethodPost:
		var request struct {
			ModelID string `json:"model_id"`
		}
		if err := decodeJSON(r.Body, &request); err != nil && err != io.EOF {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		request.ModelID = strings.TrimSpace(request.ModelID)
		if request.ModelID == "" {
			request.ModelID = s.cfg.Models[0].ID
		}
		if _, ok := s.cfg.ModelByID(request.ModelID); !ok {
			writeError(w, http.StatusBadRequest, "unknown model")
			return
		}
		conversation, err := s.store.CreateConversation(userID, request.ModelID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, conversation)
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleConversationByID(w http.ResponseWriter, r *http.Request) {
	conversationID, suffix := splitConversationPath(r.URL.Path)
	if conversationID == "" {
		http.NotFound(w, r)
		return
	}
	conversation, err := s.store.GetConversation(conversationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if conversation == nil || conversation.UserID != userIDFromContext(r.Context()) {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}

	if suffix == "/messages" {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		messages, err := s.store.ListMessages(conversationID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for i := range messages {
			if messages[i].Role == "assistant" {
				messages[i].ThinkContent = chat.ExtractThink(messages[i].ContentRaw)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"conversation": conversation, "messages": messages})
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var request struct {
			Title   string `json:"title"`
			ModelID string `json:"model_id"`
		}
		if err := decodeJSON(r.Body, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		title := strings.TrimSpace(request.Title)
		if title == "" {
			title = conversation.Title
		}
		modelID := strings.TrimSpace(request.ModelID)
		if modelID == "" {
			modelID = conversation.ModelID
		}
		if _, ok := s.cfg.ModelByID(modelID); !ok {
			writeError(w, http.StatusBadRequest, "unknown model")
			return
		}
		if err := s.store.UpdateConversation(conversation.ID, title, modelID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		updated, _ := s.store.GetConversation(conversation.ID)
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeleteConversation(conversation.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		writeMethodNotAllowed(w)
	}
}

func (s *Server) handleMessageByID(w http.ResponseWriter, r *http.Request) {
	messageIDStr := strings.TrimPrefix(r.URL.Path, "/api/messages/")
	if messageIDStr == "" {
		http.NotFound(w, r)
		return
	}
	var messageID int64
	if _, err := fmt.Sscanf(messageIDStr, "%d", &messageID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid message id")
		return
	}

	message, err := s.store.GetMessage(messageID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if message == nil {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}

	conversation, err := s.store.GetConversation(message.ConversationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if conversation == nil || conversation.UserID != userIDFromContext(r.Context()) {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}

	if r.Method != http.MethodDelete {
		writeMethodNotAllowed(w)
		return
	}

	truncate := r.URL.Query().Get("truncate") == "true"
	if truncate {
		err = s.store.DeleteMessagesFrom(conversation.ID, messageID)
	} else {
		err = s.store.DeleteMessage(messageID)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePartialAssistantMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var request struct {
		ConversationID string `json:"conversation_id"`
		ModelID        string `json:"model_id"`
		Content        string `json:"content"`
		ThinkContent   string `json:"think_content"`
	}
	if err := decodeJSON(r.Body, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	message, err := s.chat.SavePartialAssistant(
		r.Context(),
		userIDFromContext(r.Context()),
		strings.TrimSpace(request.ConversationID),
		strings.TrimSpace(request.ModelID),
		request.Content,
		request.ThinkContent,
	)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	conversation, _ := s.store.GetConversation(strings.TrimSpace(request.ConversationID))
	writeJSON(w, http.StatusOK, map[string]any{
		"message":      message,
		"conversation": conversation,
	})
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var request struct {
		ConversationID string `json:"conversation_id"`
		ModelID        string `json:"model_id"`
		Message        string `json:"message"`
		Stream         bool   `json:"stream"`
		EnableThinking bool   `json:"enable_thinking"`
		EnableSearch   bool   `json:"enable_search"`
	}
	if err := decodeJSON(r.Body, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	acceptStream := strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")
	if request.Stream || acceptStream {
		s.handleChatCompletionsStream(w, r, strings.TrimSpace(request.ConversationID), strings.TrimSpace(request.ModelID), request.Message, chat.GenerationOptions{
			EnableThinking: request.EnableThinking,
			EnableSearch:   request.EnableSearch,
		})
		return
	}
	message, err := s.chat.SendMessage(r.Context(), userIDFromContext(r.Context()), strings.TrimSpace(request.ConversationID), strings.TrimSpace(request.ModelID), request.Message, chat.GenerationOptions{
		EnableThinking: request.EnableThinking,
		EnableSearch:   request.EnableSearch,
	})
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	if message != nil && message.Role == "assistant" && message.ThinkContent == "" {
		message.ThinkContent = chat.ExtractThink(message.ContentRaw)
	}
	conversation, _ := s.store.GetConversation(strings.TrimSpace(request.ConversationID))
	writeJSON(w, http.StatusOK, map[string]any{
		"message":      message,
		"conversation": conversation,
	})
}

func (s *Server) handleChatCompletionsStream(w http.ResponseWriter, r *http.Request, conversationID, modelID, messageText string, options chat.GenerationOptions) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	message, err := s.chat.StreamMessage(r.Context(), userIDFromContext(r.Context()), conversationID, modelID, messageText, options, func(chunk chat.StreamChunk) error {
		if err := writeSSE(w, "delta", chunk); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	})
	if err != nil {
		_ = writeSSE(w, "error", map[string]string{"error": err.Error()})
		flusher.Flush()
		return
	}
	conversation, _ := s.store.GetConversation(conversationID)
	_ = writeSSE(w, "done", map[string]any{
		"message":      message,
		"conversation": conversation,
	})
	flusher.Flush()
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := s.sessions.Authenticate(r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
	})
}

func userIDFromContext(ctx context.Context) string {
	userID, _ := ctx.Value(userIDKey).(string)
	return userID
}

func splitConversationPath(requestPath string) (string, string) {
	trimmed := strings.TrimPrefix(requestPath, "/api/conversations/")
	if trimmed == requestPath || trimmed == "" {
		return "", ""
	}
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], "/" + parts[1]
}

func decodeJSON(body io.Reader, target any) error {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeSSE(w http.ResponseWriter, event string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", string(body)); err != nil {
		return err
	}
	return nil
}

func spaHandler(staticFS http.FileSystem) http.Handler {
	fileServer := http.FileServer(staticFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleaned := path.Clean(r.URL.Path)
		if cleaned == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}
		if file, err := staticFS.Open(strings.TrimPrefix(cleaned, "/")); err == nil {
			_ = file.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		index, err := staticFS.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer index.Close()
		stat, err := index.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, "index.html", stat.ModTime(), index)
	})
}
