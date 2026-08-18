package transcript

import "strings"

// Role is a chat message role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
	RoleSummary   Role = "summary"
)

// Message is the internal transcript type used by STM / compaction / extraction.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

func System(content string) Message { return Message{Role: RoleSystem, Content: content} }
func User(content string) Message   { return Message{Role: RoleUser, Content: content} }
func Assistant(content string) Message {
	return Message{Role: RoleAssistant, Content: content}
}

// Summary builds a compaction summary message (prefixed for model clarity).
func Summary(content string) Message {
	content = strings.TrimSpace(content)
	if content == "" {
		return Message{Role: RoleSummary, Content: "[会话摘要]"}
	}
	if !strings.HasPrefix(content, "[会话摘要]") {
		content = "[会话摘要]\n" + content
	}
	return Message{Role: RoleSummary, Content: content}
}
