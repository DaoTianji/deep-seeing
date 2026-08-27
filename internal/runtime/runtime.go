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
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	callbackutils "github.com/cloudwego/eino/utils/callbacks"

	deepagent "deep-seeing/internal/agent"
	"deep-seeing/internal/attention"
	"deep-seeing/internal/body"
	"deep-seeing/internal/compaction"
	"deep-seeing/internal/contextsource"
	"deep-seeing/internal/identity"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/observe"
	"deep-seeing/internal/prompt"
	"deep-seeing/internal/transcript"
)

// Service runs one conversational turn with STM + SideQuery + Eino + PostTurn.
type Service struct {
	Scope          identity.TenantScope
	SessionID      string
	STM            memory.SessionStore
	SideQuery      memory.SideQuerySelector
	RecallMode     RecallMode
	Norms          *NormSnapshotCache
	TaskContext    TaskContextProvider
	Attention      *attention.SessionStore
	Assembler      prompt.Assembler
	Compactor      compaction.Compactor
	Agent          *react.Agent
	PostTurn       memory.PostTurnExtractor
	Soul           string
	Origin         string // may be empty after first_boot
	Capability     string
	FirstBoot      bool
	Model          string
	Journal        *observe.Journal
	ContextSources map[contextsource.Source]string

	mu     sync.Mutex
	sysMsg string
}

// Options configures a Service.
type Options struct {
	Scope          identity.TenantScope
	SessionID      string
	STM            memory.SessionStore
	SideQuery      memory.SideQuerySelector
	RecallMode     RecallMode
	Norms          *NormSnapshotCache
	TaskContext    TaskContextProvider
	Attention      *attention.SessionStore
	Assembler      prompt.Assembler
	Compactor      compaction.Compactor
	Agent          *react.Agent
	PostTurn       memory.PostTurnExtractor
	Soul           string
	Origin         string
	Capability     string
	FirstBoot      bool
	Model          string
	Journal        *observe.Journal
	ContextSources map[contextsource.Source]string
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
	recallMode := opt.RecallMode
	if recallMode != RecallModeAgent {
		recallMode = RecallModeLegacy
	}
	norms := opt.Norms
	if recallMode == RecallModeAgent && norms == nil {
		norms = NewNormSnapshotCache(nil, scope)
	}
	return &Service{
		Scope:          scope,
		SessionID:      sessionID,
		STM:            opt.STM,
		SideQuery:      opt.SideQuery,
		RecallMode:     recallMode,
		Norms:          norms,
		TaskContext:    opt.TaskContext,
		Attention:      opt.Attention,
		Assembler:      assembler,
		Compactor:      comp,
		Agent:          opt.Agent,
		PostTurn:       post,
		Soul:           persona,
		Origin:         origin,
		Capability:     opt.Capability,
		FirstBoot:      opt.FirstBoot,
		Model:          opt.Model,
		Journal:        opt.Journal,
		ContextSources: cloneContextSourceStates(opt.ContextSources),
	}, nil
}

// SystemProvider returns the latest assembled system prompt for MessageModifier.
func (s *Service) SystemProvider() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sysMsg
}

// WarmNorm loads the session snapshot before the first agent turn.
func (s *Service) WarmNorm(ctx context.Context) error {
	if s == nil || s.RecallMode != RecallModeAgent || s.Norms == nil {
		return nil
	}
	_, err := s.Norms.Snapshot(ctx)
	return err
}

// InvalidateNorm refreshes the complete Bond background on the next turn.
func (s *Service) InvalidateNorm() {
	if s != nil && s.Norms != nil {
		s.Norms.Invalidate()
	}
}

// TurnResult is the assistant text for one turn.
type TurnResult struct {
	Answer     string
	TokenUsage observe.TokenUsageTrace
}

// TurnHooks exposes observable turn activity without exposing hidden reasoning.
type TurnHooks struct {
	WriteDelta          func(string)
	OnToolStart         func(string)
	OnRecallSearch      func(observe.RecallSearchTrace)
	OnRecallRead        func(observe.RecallReadTrace)
	OnRecallEvidence    func(observe.RecallEvidenceTrace)
	OnTaskContext       func(observe.TaskContextTrace)
	OnContextExpand     func(observe.TaskContextExpansionTrace)
	OnContextFocus      func(observe.TaskContextFocusTrace)
	OnContextSource     func(observe.ContextSourceTrace)
	OnContextCandidate  func(observe.ContextCandidateTrace)
	OnContextRead       func(observe.ContextReadTrace)
	OnContextUse        func(observe.ContextUseTrace)
	OnAttentionSnapshot func(attention.Snapshot)
	OnAttentionDecision func(observe.AttentionDecisionTrace)
}

// StreamTurn prepares context, runs Eino ReAct streaming, updates STM, and schedules extraction.
// writeDelta is called for each content chunk (may be empty).
func (s *Service) StreamTurn(ctx context.Context, userText string, writeDelta func(string), onToolStart func(string)) (TurnResult, error) {
	return s.StreamTurnWithHooks(ctx, userText, TurnHooks{
		WriteDelta: writeDelta, OnToolStart: onToolStart,
	})
}

// StreamTurnWithHooks runs one turn and streams structured external activity.
func (s *Service) StreamTurnWithHooks(ctx context.Context, userText string, hooks TurnHooks) (TurnResult, error) {
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

	turnCtx, attentionCollector := observe.WithAttentionHooks(ctx, observe.AttentionHooks{
		OnSnapshot: hooks.OnAttentionSnapshot, OnDecision: hooks.OnAttentionDecision,
	})
	turnCtx, contextCollector := observe.WithContextHooks(turnCtx, observe.ContextHooks{
		OnSource: hooks.OnContextSource, OnCandidate: hooks.OnContextCandidate,
		OnRead: hooks.OnContextRead, OnUse: hooks.OnContextUse,
	})
	turnCtx, recallCollector := observe.WithRecallHooks(turnCtx, observe.RecallHooks{
		OnSearch: hooks.OnRecallSearch, OnRead: hooks.OnRecallRead, OnEvidence: hooks.OnRecallEvidence,
	})
	turnCtx, taskContextCollector := observe.WithTaskContextHooks(turnCtx, observe.TaskContextHooks{
		OnExpand: hooks.OnContextExpand, OnFocus: hooks.OnContextFocus,
	})
	attentionText := ""
	if s.RecallMode == RecallModeAgent && s.Attention != nil {
		snapshot := s.Attention.BeginTurn(s.SessionID)
		attentionText = formatAttentionSnapshot(snapshot)
		observe.RecordAttentionSnapshot(turnCtx, snapshot)
	}

	if s.RecallMode == RecallModeAgent {
		for _, source := range contextsource.OrderedSources() {
			if state, ok := s.ContextSources[source]; ok {
				observe.RecordContextSource(turnCtx, observe.ContextSourceTrace{Source: source, State: state})
			}
		}
	}
	turnMemory := s.prepareTurnMemory(turnCtx, userText)
	if s.RecallMode == RecallModeAgent {
		state := "available"
		if turnMemory.bondPlaceholder {
			state = "unavailable"
		}
		observe.RecordContextSource(turnCtx, observe.ContextSourceTrace{
			Source: contextsource.Bond, State: state, Version: fmt.Sprint(turnMemory.normVersion),
		})
	}
	taskContextText := ""
	var taskContextTrace *observe.TaskContextTrace
	if snapshot, ok := s.prepareTaskContext(turnCtx); ok {
		taskContextText = snapshot.PromptText()
		trace := snapshot.Trace()
		taskContextTrace = &trace
		recordTaskContextSources(turnCtx, trace)
		if hooks.OnTaskContext != nil {
			hooks.OnTaskContext(trace)
		}
	}
	sysMsgs, err := s.Assembler.BuildSystemMessages(ctx, prompt.AssembleInput{
		Scope:            s.Scope,
		SessionID:        s.SessionID,
		Soul:             s.Soul,
		Capability:       s.Capability,
		OriginContext:    s.Origin,
		BondNorm:         turnMemory.bondNorm,
		MemoryRecall:     prompt.FormatMemoryRecall(turnMemory.recallLines),
		TaskContext:      taskContextText,
		AttentionContext: attentionText,
		RecallGuidance:   turnMemory.recallGuidance,
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
	tokenCounter := &turnTokenCounter{}
	toolCB := func(name string) {
		toolStarts = append(toolStarts, name)
		if hooks.OnToolStart != nil {
			hooks.OnToolStart(name)
		}
	}
	opts = append(opts, agent.WithComposeOptions(compose.WithCallbacks(toolStartCallback(toolCB), tokenUsageCallback(tokenCounter))))
	sr, err := s.Agent.Stream(turnCtx, einoMsgs, opts...)
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
			if hooks.WriteDelta != nil {
				hooks.WriteDelta(msg.Content)
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
		if hooks.WriteDelta != nil {
			hooks.WriteDelta(note)
		}
		final = strings.TrimSpace(final + note)
		log.Printf("agent recv soft-complete: %v", streamErr)
	}

	if err := s.STM.Append(s.SessionID, transcript.User(userText), transcript.Assistant(final)); err != nil {
		log.Printf("stm append: %v", err)
	}

	if s.Journal != nil {
		_ = s.Journal.Append(observe.TurnTrace{
			SessionID:          s.SessionID,
			AgentID:            s.Scope.AgentID,
			PersonID:           s.Scope.PersonID(),
			ModelVersion:       s.Model,
			RuntimeVer:         body.ToolsetVersion,
			RecallMode:         string(s.RecallMode),
			UserText:           observe.Preview(userText, 120),
			NormVersion:        turnMemory.normVersion,
			RecallIDs:          turnMemory.recallIDs,
			RecallSearches:     recallCollector.Searches(),
			RecallReads:        recallCollector.Reads(),
			RecallEvidence:     recallCollector.Evidence(),
			TaskContext:        taskContextTrace,
			ContextExpands:     taskContextCollector.Expansions(),
			ContextFocus:       taskContextCollector.Focus(),
			ContextSources:     contextCollector.Sources(),
			ContextCandidates:  contextCollector.Candidates(),
			ContextReads:       contextCollector.Reads(),
			ContextUses:        contextCollector.Uses(),
			Attention:          attentionCollector.Snapshot(),
			AttentionDecisions: attentionCollector.Decisions(),
			BondSlots:          turnMemory.bondSlots,
			BondItemIDs:        turnMemory.bondItemIDs,
			BondPlaceholder:    turnMemory.bondPlaceholder,
			SceneIDs:           turnMemory.sceneIDs,
			ToolStarts:         toolStarts,
			Errors:             turnErrors,
			AnswerPreview:      observe.Preview(final, 200),
			TokenUsage:         tokenCounter.Snapshot(),
		})
	}

	go func() {
		bg := context.Background()
		if err := s.PostTurn.AfterTurn(bg, s.Scope, s.SessionID, userText, final); err != nil {
			log.Printf("post-turn extract: %v", err)
		}
	}()

	return TurnResult{Answer: final, TokenUsage: tokenCounter.Snapshot()}, nil
}

func recordTaskContextSources(ctx context.Context, trace observe.TaskContextTrace) {
	observe.RecordContextSource(ctx, observe.ContextSourceTrace{Source: contextsource.Workspace, State: trace.WorkspaceStatus})
	observe.RecordContextSource(ctx, observe.ContextSourceTrace{Source: contextsource.Intent, State: trace.IntentStatus})
	if len(trace.WorkspaceIDs) > 0 {
		observe.RecordContextCandidate(ctx, observe.ContextCandidateTrace{
			Source: contextsource.Workspace, Operation: "snapshot", ResultIDs: trace.WorkspaceIDs,
		})
	}
	if len(trace.IntentIDs) > 0 {
		observe.RecordContextCandidate(ctx, observe.ContextCandidateTrace{
			Source: contextsource.Intent, Operation: "snapshot", ResultIDs: trace.IntentIDs,
		})
	}
}

func cloneContextSourceStates(in map[contextsource.Source]string) map[contextsource.Source]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[contextsource.Source]string, len(in))
	for source, state := range in {
		out[source] = state
	}
	return out
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

type turnTokenCounter struct {
	mu    sync.Mutex
	usage observe.TokenUsageTrace
}

func (c *turnTokenCounter) Add(usage *model.TokenUsage) {
	if c == nil || usage == nil {
		return
	}
	c.mu.Lock()
	c.usage.PromptTokens += usage.PromptTokens
	c.usage.CompletionTokens += usage.CompletionTokens
	c.usage.ReasoningTokens += usage.CompletionTokensDetails.ReasoningTokens
	c.usage.TotalTokens += usage.TotalTokens
	c.mu.Unlock()
}

func (c *turnTokenCounter) Snapshot() observe.TokenUsageTrace {
	if c == nil {
		return observe.TokenUsageTrace{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.usage
}

func tokenUsageCallback(counter *turnTokenCounter) callbacks.Handler {
	return callbackutils.NewHandlerHelper().ChatModel(&callbackutils.ModelCallbackHandler{
		OnEnd: func(ctx context.Context, _ *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
			if output != nil {
				counter.Add(output.TokenUsage)
			}
			return ctx
		},
		OnEndWithStreamOutput: func(ctx context.Context, _ *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
			if output == nil {
				return ctx
			}
			defer output.Close()
			for {
				chunk, err := output.Recv()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					break
				}
				if chunk != nil {
					counter.Add(chunk.TokenUsage)
				}
			}
			return ctx
		},
	}).Handler()
}
