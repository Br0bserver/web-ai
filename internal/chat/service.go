package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"web-ai/internal/config"
	"web-ai/internal/openai"
	"web-ai/internal/render"
	"web-ai/internal/store"
)

type Service struct {
	cfg      *config.Config
	store    *store.Store
	renderer *render.Renderer
	client   *openai.Client
}

type StreamChunk struct {
	ContentDelta string `json:"content_delta,omitempty"`
	ThinkDelta   string `json:"think_delta,omitempty"`
	ThinkPreview string `json:"think_preview,omitempty"`
}

func NewService(cfg *config.Config, st *store.Store, renderer *render.Renderer, client *openai.Client) *Service {
	return &Service{cfg: cfg, store: st, renderer: renderer, client: client}
}

func (s *Service) SendMessage(ctx context.Context, userID, conversationID, modelID, input string) (*store.Message, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("message is required")
	}
	conversation, err := s.store.GetConversation(conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil || conversation.UserID != userID {
		return nil, fmt.Errorf("conversation not found")
	}
	if modelID == "" {
		modelID = conversation.ModelID
	}
	if _, ok := s.cfg.ModelByID(modelID); !ok {
		return nil, fmt.Errorf("unknown model")
	}
	if conversation.ModelID != modelID {
		if err := s.store.UpdateConversation(conversation.ID, conversation.Title, modelID); err != nil {
			return nil, err
		}
		conversation.ModelID = modelID
	}

	history, err := s.store.ListMessages(conversationID)
	if err != nil {
		return nil, err
	}
	firstExchange := len(history) == 0

	userHTML := s.renderer.RenderMarkdown(input)
	if _, err := s.store.AddMessage(conversationID, "user", modelID, input, userHTML); err != nil {
		return nil, err
	}

	upstreamMessages := make([]openai.Message, 0, len(history)+1)
	for _, message := range history {
		upstreamMessages = append(upstreamMessages, openai.Message{Role: message.Role, Content: message.ContentRaw})
	}
	upstreamMessages = append(upstreamMessages, openai.Message{Role: "user", Content: input})

	response, err := s.client.Chat(ctx, modelID, upstreamMessages)
	if err != nil {
		return nil, err
	}
	thinkText, answerText := splitThinkAndAnswer(response, "")
	if answerText == "" {
		answerText = strings.TrimSpace(response)
	}

	assistantHTML := s.renderer.RenderMarkdown(answerText)
	assistantMessage, err := s.store.AddMessage(conversationID, "assistant", modelID, response, assistantHTML)
	if err != nil {
		return nil, err
	}
	assistantMessage.ThinkContent = thinkText

	if firstExchange && s.cfg.TitleModel.ID != "" {
		go s.generateTitle(conversationID, input, answerText)
	}

	return assistantMessage, nil
}

func (s *Service) StreamMessage(ctx context.Context, userID, conversationID, modelID, input string, onChunk func(StreamChunk) error) (*store.Message, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("message is required")
	}
	conversation, err := s.store.GetConversation(conversationID)
	if err != nil {
		return nil, err
	}
	if conversation == nil || conversation.UserID != userID {
		return nil, fmt.Errorf("conversation not found")
	}
	if modelID == "" {
		modelID = conversation.ModelID
	}
	if _, ok := s.cfg.ModelByID(modelID); !ok {
		return nil, fmt.Errorf("unknown model")
	}
	if conversation.ModelID != modelID {
		if err := s.store.UpdateConversation(conversation.ID, conversation.Title, modelID); err != nil {
			return nil, err
		}
		conversation.ModelID = modelID
	}

	history, err := s.store.ListMessages(conversationID)
	if err != nil {
		return nil, err
	}
	firstExchange := len(history) == 0

	userHTML := s.renderer.RenderMarkdown(input)
	if _, err := s.store.AddMessage(conversationID, "user", modelID, input, userHTML); err != nil {
		return nil, err
	}

	upstreamMessages := make([]openai.Message, 0, len(history)+1)
	for _, message := range history {
		upstreamMessages = append(upstreamMessages, openai.Message{Role: message.Role, Content: message.ContentRaw})
	}
	upstreamMessages = append(upstreamMessages, openai.Message{Role: "user", Content: input})

	var liveThink strings.Builder
	content, thinking, err := s.client.ChatStream(ctx, modelID, upstreamMessages, func(delta openai.StreamDelta) error {
		if onChunk == nil {
			return nil
		}
		chunk := StreamChunk{ContentDelta: delta.ContentDelta, ThinkDelta: delta.ThinkDelta}
		if delta.ThinkDelta != "" {
			liveThink.WriteString(delta.ThinkDelta)
			chunk.ThinkPreview = tailLines(liveThink.String(), 5)
		}
		if chunk.ContentDelta == "" && chunk.ThinkDelta == "" {
			return nil
		}
		return onChunk(chunk)
	})
	if err != nil {
		return nil, err
	}

	thinkText, answerText := splitThinkAndAnswer(content, thinking)
	if answerText == "" {
		answerText = strings.TrimSpace(content)
	}
	finalRaw := answerText
	if thinkText != "" {
		finalRaw = "<think>\n" + thinkText + "\n</think>\n\n" + answerText
	}

	assistantHTML := s.renderer.RenderMarkdown(answerText)
	assistantMessage, err := s.store.AddMessage(conversationID, "assistant", modelID, finalRaw, assistantHTML)
	if err != nil {
		return nil, err
	}
	assistantMessage.ThinkContent = thinkText

	if firstExchange && s.cfg.TitleModel.ID != "" {
		go s.generateTitle(conversationID, input, answerText)
	}

	return assistantMessage, nil
}

func (s *Service) generateTitle(conversationID, firstUser, firstAssistant string) {
	prompt := strings.TrimSpace(s.cfg.TitleModel.Prompt)
	if prompt == "" {
		prompt = config.DefaultTitlePrompt
	}
	content := "First user message:\n" + firstUser + "\n\nFirst assistant reply:\n" + firstAssistant
	for attempt := 0; attempt < 3; attempt++ {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		title, err := s.client.Chat(timeoutCtx, s.cfg.TitleModel.ID, []openai.Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: content},
		})
		cancel()
		if err != nil {
			time.Sleep(time.Duration(attempt+1) * 800 * time.Millisecond)
			continue
		}
		title = sanitizeTitle(title)
		if title == "" {
			return
		}
		_ = s.store.UpdateConversationTitle(conversationID, title)
		return
	}
}

func ExtractThink(content string) string {
	think, _ := splitThinkAndAnswer(content, "")
	return think
}

func splitThinkAndAnswer(content, thinking string) (string, string) {
	raw := strings.TrimSpace(content)
	answer := raw
	var thinkSegments []string
	if strings.TrimSpace(thinking) != "" {
		thinkSegments = append(thinkSegments, strings.TrimSpace(thinking))
	}
	for {
		start := strings.Index(strings.ToLower(answer), "<think>")
		if start < 0 {
			break
		}
		end := strings.Index(strings.ToLower(answer[start+7:]), "</think>")
		if end < 0 {
			segment := strings.TrimSpace(answer[start+7:])
			if segment != "" {
				thinkSegments = append(thinkSegments, segment)
			}
			answer = strings.TrimSpace(answer[:start])
			break
		}
		segment := strings.TrimSpace(answer[start+7 : start+7+end])
		if segment != "" {
			thinkSegments = append(thinkSegments, segment)
		}
		answer = strings.TrimSpace(answer[:start] + " " + answer[start+7+end+8:])
	}
	thinkText := strings.TrimSpace(strings.Join(thinkSegments, "\n"))
	answer = strings.TrimSpace(answer)
	return thinkText, answer
}

func tailLines(text string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) <= maxLines {
		return trimmed
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

func sanitizeTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'`“”‘’「」[]()")
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		return ""
	}
	if len([]rune(title)) > 48 {
		title = string([]rune(title)[:48])
	}
	if title == store.DefaultConversationTitle {
		return ""
	}
	return title
}
