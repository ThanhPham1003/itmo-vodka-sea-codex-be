package services

import (
	"context"
	"sea-codex-server/internal/assistant/dto"
	"sync"
)

type IAssistantService interface {
	Context(context.Context, string, string) (dto.ContextResponse, error)
	Article(context.Context, string, string) (dto.ArticleResponse, error)
	Chat(context.Context, dto.ChatRequest) (dto.ChatResponse, error)
}

var registry struct {
	sync.RWMutex
	service IAssistantService
}

func RegisterAssistantService(s IAssistantService) {
	registry.Lock()
	defer registry.Unlock()
	registry.service = s
}
func AssistantService() IAssistantService {
	registry.RLock()
	defer registry.RUnlock()
	return registry.service
}
