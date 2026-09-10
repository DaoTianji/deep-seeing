package modelopts

import (
	"net/url"
	"strings"
)

// Extra enables reasoning using each DeepSeek provider's own parameter schema. The current Eino
// adapter round-trips reasoning_content across tool calls; it is not published
// in our public trace. Do not apply provider-specific options to other gateways.
func Extra(base, model string) map[string]any {
	u, err := url.Parse(base)
	if err == nil && strings.EqualFold(u.Hostname(), "api.siliconflow.cn") && strings.HasPrefix(model, "deepseek-ai/DeepSeek-V4-") {
		return map[string]any{"enable_thinking": true, "reasoning_effort": "high"}
	}
	if err == nil && u.Scheme == "https" && u.User == nil && strings.EqualFold(u.Hostname(), "api.deepseek.com") {
		switch model {
		case "deepseek-v4-pro", "deepseek-flash", "deepseek-v4-flash":
			return map[string]any{"thinking": map[string]any{"type": "enabled"}, "reasoning_effort": "high"}
		}
	}
	return nil
}
