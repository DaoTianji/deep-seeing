package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"deep-seeing/internal/identity"
)

// GenerativeDreamer recombines unresolved tensions into explicitly fictional
// hypotheses. Its only durable output is a generated ReflectionSeed; it has no
// Proposal, Bond, Self or Episode write surface.
type GenerativeDreamer struct {
	Chat  ReflectionCompleter
	Live  *ReflectionLiveStore
	Store *ReflectionStore
	Mode  ReflectionMode
}

type generativeDreamOut struct {
	WorthDreaming bool   `json:"worth_dreaming"`
	DreamFragment string `json:"dream_fragment"`
	Notes         string `json:"notes"`
	Hypotheses    []struct {
		Scope     string `json:"scope"`
		Statement string `json:"statement"`
	} `json:"hypotheses"`
}

func tentativeGeneratedStatement(raw string) string {
	statement := strings.TrimSpace(raw)
	if statement == "" {
		return ""
	}
	for _, marker := range []string{"是否", "可能", "也许", "会不会", "？", "?"} {
		if strings.Contains(statement, marker) {
			return statement
		}
	}
	return "待验证：是否可能" + strings.TrimRight(statement, "。！!？?") + "？"
}

func (g *GenerativeDreamer) Run(ctx context.Context, scope identity.TenantScope, sessionID string, trigger ReflectionTrigger) (ReflectionRun, error) {
	started := time.Now().UTC()
	mode := g.Mode
	if mode == "" {
		mode = ReflectionModeObserve
	}
	run := ReflectionRun{
		ID: "grun_" + strings.ReplaceAll(uuid.NewString(), "-", ""), PersonID: scope.PersonID(),
		SessionID: sessionID, Mode: mode, Trigger: trigger, StartedAt: started,
	}
	g.Live.Update(run, "generating", true)
	finish := func(err error) (ReflectionRun, error) {
		run.CompletedAt = time.Now().UTC()
		run.Duration = run.CompletedAt.Sub(run.StartedAt)
		if err != nil {
			run.Error = truncate(err.Error(), 240)
		}
		if g.Store != nil {
			if saved, appendErr := g.Store.AppendRun(run); appendErr == nil {
				run = saved
			} else if err == nil {
				err = appendErr
			}
		}
		g.Live.Update(run, "completed", false)
		return run, err
	}
	if err := scope.Validate(); err != nil {
		return finish(err)
	}
	if g.Store == nil || g.Chat == nil {
		run.NoChange, run.Notes = true, "Generative Dream unavailable."
		return finish(nil)
	}
	seeds, err := g.Store.List(ctx, scope, scope.PersonID(), false, 50)
	if err != nil {
		return finish(err)
	}
	var tensions []ReflectionSeed
	for _, seed := range seeds {
		if seed.Generated {
			continue
		}
		if seed.Scope == ReflectionScopeTension || seed.Status == ReflectionSeedDeferred {
			tensions = append(tensions, seed)
		}
	}
	if len(tensions) == 0 && trigger != ReflectionTriggerManual {
		run.NoChange, run.Notes = true, "No unresolved tension."
		return finish(nil)
	}
	if len(tensions) == 0 {
		for _, seed := range seeds {
			if !seed.Generated {
				tensions = append(tensions, seed)
			}
			if len(tensions) == 3 {
				break
			}
		}
	}
	if len(tensions) == 0 {
		run.NoChange, run.Notes = true, "No eligible reflection material."
		return finish(nil)
	}
	for _, seed := range tensions {
		run.SeedIDs = append(run.SeedIDs, seed.ID)
	}
	system := strings.TrimSpace(`
你位于 Deep-Seeing 的生成式 Dream 沙箱。这里允许联想、隐喻和重组，但所有输出都只是想象假设，不是事实、Episode 或长期认识。

只围绕输入的未解决张力提出少量新视角。不要补写从未发生的事件，不要把假设说成记忆，不要提出直接修改 Bond/Self/Principle 的动作。
scope 仅可为 bond|self_pattern|tension|principle_candidate。输出短公开片段和可在未来由真实经历验证的问题。允许 worth_dreaming=false。只输出 JSON。

{"worth_dreaming":false,"dream_fragment":"","hypotheses":[{"scope":"tension","statement":"一个明确标为可能性的短问题"}],"notes":"No change."}`)
	raw, err := g.Chat.Complete(ctx, system, mustJSON(map[string]any{"unresolved_tensions": seedCards(tensions)}))
	if err != nil {
		return finish(err)
	}
	var out generativeDreamOut
	if err := json.Unmarshal([]byte(stripJSONFence(raw)), &out); err != nil {
		return finish(fmt.Errorf("parse generative dream: %w", err))
	}
	if !out.WorthDreaming {
		run.NoChange, run.Notes = true, firstNonEmpty(out.Notes, "No change.")
		return finish(nil)
	}
	run.GenerativeNote = truncate(out.DreamFragment, 500)
	run.Notes = truncate(out.Notes, 240)
	for _, hypothesis := range out.Hypotheses {
		statement := tentativeGeneratedStatement(hypothesis.Statement)
		if statement == "" {
			continue
		}
		seed, createErr := g.Store.Create(ctx, scope, ReflectionSeedWrite{
			SessionID: sessionID, Scope: ReflectionScope(hypothesis.Scope), Statement: statement,
			SourceType: ReflectionSourceGenerated, ExperienceModes: []ExperienceMode{ExperienceSelfReflection}, Generated: true,
		})
		if createErr != nil {
			continue
		}
		run.GeneratedSeedIDs = append(run.GeneratedSeedIDs, seed.ID)
		run.GeneratedSeeds = append(run.GeneratedSeeds, GeneratedReflectionSeed{ID: seed.ID, Scope: seed.Scope, Statement: seed.Statement, Generated: true})
	}
	if len(run.GeneratedSeedIDs) == 0 {
		run.NoChange = true
	}
	return finish(nil)
}
