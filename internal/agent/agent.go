package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"deep-seeing/internal/soul"
)

// DefaultSoul loads seed Soul text (no specific person inside).
func DefaultSoul() string {
	return soul.MustLoad("")
}

// Config wires the model gateway.
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
}

// New builds a ReAct agent. systemProvider, when non-empty, replaces the default system message
// for each model call (used by runtime after SideQuery assembly).
func New(ctx context.Context, cfg Config, toolList []tool.BaseTool, systemProvider func() string) (*react.Agent, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("缺少 OPENAI_API_KEY")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("缺少 OPENAI_MODEL")
	}
	if len(toolList) == 0 {
		return nil, fmt.Errorf("tools required")
	}

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  cfg.APIKey,
		BaseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		Model:   cfg.Model,
		Timeout: 240 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("create chat model: %w", err)
	}

	return react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: toolList,
		},
		MessageModifier: func(_ context.Context, input []*schema.Message) []*schema.Message {
			sys := DefaultSoul()
			if systemProvider != nil {
				if s := strings.TrimSpace(systemProvider()); s != "" {
					sys = s
				}
			}
			out := make([]*schema.Message, 0, len(input)+1)
			out = append(out, schema.SystemMessage(sys))
			out = append(out, input...)
			return out
		},
		MaxStep: 16,
	})
}

// ConfigFromEnv loads gateway settings.
func ConfigFromEnv() Config {
	return Config{
		APIKey:  firstEnv("OPENAI_API_KEY", "AI_GATEWAY_API_KEY"),
		BaseURL: firstEnv("OPENAI_BASE_URL", "AI_GATEWAY_BASE_URL"),
		Model:   firstEnv("OPENAI_MODEL", "AI_GATEWAY_MODEL"),
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}
