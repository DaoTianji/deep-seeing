package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	deepagent "deep-seeing/internal/agent"
	"deep-seeing/internal/body"
	"deep-seeing/internal/compaction"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
	"deep-seeing/internal/prompt"
	"deep-seeing/internal/transcript"
)

// Service runs one conversational turn with STM + SideQuery + Eino + PostTurn.
type Service struct {
	Scope      identity.TenantScope
	SessionID  string
	STM        memory.SessionStore
	SideQuery  memory.SideQuerySelector
	Assembler  prompt.Assembler
	Compactor  compaction.Compactor
	Agent      *react.Agent
	PostTurn   memory.PostTurnExtractor
	Soul       string
	Origin     string // may be empty after first_boot
	Capability string
	FirstBoot  bool
	Model      string
	Journal    *observe.Journal

	mu     sync.Mutex
	sysMsg string
}

// Options configures a Service.
type Options struct {
	Scope      identity.TenantScope
	SessionID  string
	STM        memory.SessionStore
	SideQuery  memory.SideQuerySelector
	Assembler  prompt.Assembler
	Compactor  compaction.Compactor
	Agent      *react.Agent
	PostTurn   memory.PostTurnExtractor
	Soul       string
	Origin     string
	Capability string
	FirstBoot  bool
	Model      string
	Journal    *observe.Journal
}

// New builds a runtime service.
func New(opt Options) (*Service, error) {
	if opt.STM == nil {
		return nil, fmt.Errorf("stm required")
	}
	if opt.Agent == nil {
		return nil, fmt.Errorf("agent required")
	}
	scope := opt.Scope
	if err := scope.Validate(); err != nil {
		scope = identity.LocalCLI()
	}
	sessionID := strings.TrimSpace(opt.SessionID)
	if sessionID == "" {
		sessionID = "default"
	}
	assembler := opt.Assembler
	if assembler == nil {
		assembler = prompt.DefaultAssembler{}
	}
	comp := opt.Compactor
	if comp == nil {
		comp = compaction.NoopCompactor{}
	}
	post := opt.PostTurn
	if post == nil {
		post = memory.NoopPostTurn{}
	}
	persona := opt.Soul
	if persona == "" {
		persona = deepagent.DefaultSoul()
	}
	origin := opt.Origin
	return &Service{
		Scope:      scope,
		SessionID:  sessionID,
		STM:        opt.STM,
		SideQuery:  opt.SideQuery,
		Assembler:  assembler,
		Compactor:  comp,
		Agent:      opt.Agent,
		PostTurn:   post,
		Soul:       persona,
		Origin:     origin,
		Capability: opt.Capability,
		FirstBoot:  opt.FirstBoot,
		Model:      opt.Model,
		Journal:    opt.Journal,
	}, nil
}

// SystemProvider returns the latest assembled system prompt for MessageModifier.
func (s *Service) SystemProvider() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sysMsg
}

// TurnResult is the assistant text for one turn.
type TurnResult struct {
	Answer string
}

// StreamTurn prepares context, runs Eino ReAct streaming, updates STM, and schedules extraction.
// writeDelta is called for each content chunk (may be empty).
func (s *Service) StreamTurn(ctx context.Context, userText string, writeDelta func(string), onToolStart func(string)) (TurnResult, error) {
	userText = strings.TrimSpace(userText)
	if userText == "" {
		return TurnResult{}, fmt.Errorf("empty message")
	}

	history, err := s.STM.Get(s.SessionID)
	if err != nil {
		return TurnResult{}, fmt.Errorf("stm get: %w", err)
	}

	// Compact history only (not the current user turn), then write back when Applied.
	compactedHistory, report, err := s.Compactor.MaybeCompact(ctx, s.Scope, s.SessionID, history, 0)
	if err != nil {
		log.Printf("compact skipped: %v", err)
		compactedHistory, report = history, compaction.Report{}
	}
	if report.Applied && report.WriteBack {
		if err := s.STM.Replace(s.SessionID, compactedHistory); err != nil {
			log.Printf("stm replace after compact: %v", err)
		} else {
			history = compactedHistory
		}
	} else if report.Applied {
		history = compactedHistory
	}

	recallLines := []string{}
	var recallIDs []string
	var bondNorm string
	var bondSlots, bondItemIDs, sceneIDs []string
	var bondPlaceholder bool
	if s.SideQuery != nil {
		recs, err := s.SideQuery.SelectForTurn(ctx, s.Scope, userText, 5)
		if err != nil {
			log.Printf("side query skipped: %v", err)
		} else {
			for _, r := range recs {
				kind := r.Metadata["kind"]
				if kind == "bond" || kind == "scene_norm" {
					if bondNorm == "" {
						bondNorm = r.Content
					} else {
						bondNorm = bondNorm + "\n\n" + r.Content
					}
					if kind == "bond" {
						if r.Metadata["placeholder"] == "1" {
							bondPlaceholder = true
						}
						if slots := r.Metadata["bond_slots"]; slots != "" {
							bondSlots = strings.Split(slots, ",")
						}
						if ids := r.Metadata["bond_item_ids"]; ids != "" {
							bondItemIDs = strings.Split(ids, ",")
						}
					}
					if ids := r.Metadata["scene_ids"]; ids != "" {
						sceneIDs = append(sceneIDs, strings.Split(ids, ",")...)
					}
					if r.ID != "" {
						recallIDs = append(recallIDs, r.ID)
					}
					continue
				}
				tag := r.ID
				if tag == "" {
					tag = r.Key
				}
				if k := r.Metadata["kind"]; k != "" {
					tag = k + "/" + tag
				}
				if about := r.Metadata["about"]; about != "" {
					tag = tag + " about:" + about
				}
				recallLines = append(recallLines, fmt.Sprintf("[%s] %s", tag, r.Content))
				if r.ID != "" {
					recallIDs = append(recallIDs, r.ID)
				}
			}
		}
	}
	sysMsgs, err := s.Assembler.BuildSystemMessages(ctx, prompt.AssembleInput{
		Scope:         s.Scope,
		SessionID:     s.SessionID,
		Soul:          s.Soul,
		Capability:    s.Capability,
		OriginContext: s.Origin,
		BondNorm:      bondNorm,
		MemoryRecall:  prompt.FormatMemoryRecall(recallLines),
	})
	if err != nil {
		return TurnResult{}, err
	}
	sysContent := ""
	if len(sysMsgs) > 0 {
		sysContent = sysMsgs[0].Content
	}
	s.mu.Lock()
	s.sysMsg = sysContent
	s.mu.Unlock()

	msgs := append([]transcript.Message(nil), history...)
	msgs = append(msgs, transcript.User(userText))

	einoMsgs := toSchemaMessages(msgs)
	var toolStarts []string
	opts := []agent.AgentOption{}
	toolCB := func(name string) {
		toolStarts = append(toolStarts, name)
		if onToolStart != nil {
			onToolStart(name)
		}
	}
	opts = append(opts, agent.WithComposeOptions(compose.WithCallbacks(toolStartCallback(toolCB))))
	sr, err := s.Agent.Stream(ctx, einoMsgs, opts...)
	if err != nil {
		return TurnResult{}, fmt.Errorf("agent stream: %w", err)
	}
	defer sr.Close()

	var answer strings.Builder
	var streamErr error
	for {
		msg, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			streamErr = err
			break
		}
		if msg == nil {
			continue
		}
		if msg.Content != "" {
			answer.WriteString(msg.Content)
			if writeDelta != nil {
				writeDelta(msg.Content)
			}
		}
	}

	final := strings.TrimSpace(answer.String())
	var turnErrors []string
	if streamErr != nil {
		turnErrors = append(turnErrors, streamErr.Error())
		if final == "" {
			return TurnResult{}, fmt.Errorf("agent recv: %w", streamErr)
		}
		// Keep partial answer so a timed-out tool retry doesn't erase an otherwise useful reply.
		note := "\n\n（本轮因超时或中断结束；以上内容已保留。若需继续检索，请再发一句。）"
		if writeDelta != nil {
			writeDelta(note)
		}
		final = strings.TrimSpace(final + note)
		log.Printf("agent recv soft-complete: %v", streamErr)
	}

	if err := s.STM.Append(s.SessionID, transcript.User(userText), transcript.Assistant(final)); err != nil {
		log.Printf("stm append: %v", err)
	}

	if s.Journal != nil {
		_ = s.Journal.Append(observe.TurnTrace{
			SessionID:       s.SessionID,
			AgentID:         s.Scope.AgentID,
			PersonID:        s.Scope.PersonID(),
			ModelVersion:    s.Model,
			RuntimeVer:      body.ToolsetVersion,
			UserText:        observe.Preview(userText, 120),
			RecallIDs:       recallIDs,
			BondSlots:       bondSlots,
			BondItemIDs:     bondItemIDs,
			BondPlaceholder: bondPlaceholder,
			SceneIDs:        sceneIDs,
			ToolStarts:      toolStarts,
			Errors:          turnErrors,
			AnswerPreview:   observe.Preview(final, 200),
		})
	}

	go func() {
		bg := context.Background()
		if err := s.PostTurn.AfterTurn(bg, s.Scope, s.SessionID, userText, final); err != nil {
			log.Printf("post-turn extract: %v", err)
		}
	}()

	return TurnResult{Answer: final}, nil
}

func toSchemaMessages(msgs []transcript.Message) []*schema.Message {
	out := make([]*schema.Message, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case transcript.RoleUser:
			out = append(out, schema.UserMessage(m.Content))
		case transcript.RoleAssistant, transcript.RoleSummary:
			out = append(out, schema.AssistantMessage(m.Content, nil))
		case transcript.RoleSystem:
			out = append(out, schema.SystemMessage(m.Content))
		default:
			out = append(out, schema.UserMessage(m.Content))
		}
	}
	return out
}

func toolStartCallback(onStart func(string)) callbacks.Handler {
	builder := callbacks.NewHandlerBuilder()
	builder.OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		if info == nil {
			return ctx
		}
		name := info.Name
		if name == "" {
			name = string(info.Component)
		}
		if strings.Contains(strings.ToLower(name), "tool") || string(info.Component) == "Tool" {
			onStart(name)
		}
		return ctx
	})
	return builder.Build()
}
