package theater

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"

	"deep-seeing/internal/evals"
	"deep-seeing/internal/memory"
	"deep-seeing/internal/runtime"
)

type learningCompleter struct {
	chapterCalls int
	failCall     int
	failed       bool
	seen         map[string]int
}

func (f *learningCompleter) Complete(_ context.Context, system, input string) (string, error) {
	if strings.Contains(system, "按顺序真实阅读") {
		f.chapterCalls++
		if f.chapterCalls == f.failCall && !f.failed {
			f.failed = true
			return "", errors.New("planned chapter interruption")
		}
		var envelope struct {
			Chapter BookChapter `json:"chapter"`
			Chunks  []RoleChunk `json:"chunks"`
		}
		if err := json.Unmarshal([]byte(input), &envelope); err != nil || len(envelope.Chunks) == 0 {
			return "", errors.New("missing full chapter input")
		}
		if f.seen == nil {
			f.seen = map[string]int{}
		}
		for _, chunk := range envelope.Chunks {
			if strings.TrimSpace(chunk.Content) == "" {
				return "", errors.New("empty chunk body")
			}
			f.seen[chunk.ID]++
		}
		id := envelope.Chunks[0].ID
		out := chapterReadingOutput{
			Summary: "本章按当前时间点完成阅读，保留未知和后续修正空间。",
			Observations: []PassageObservation{
				{Kind: "explicit_feeling", Statement: "角色明确表达害怕。", ChunkIDs: []string{id}, Explicit: true, Confidence: "high"},
				{Kind: "inferred_feeling", Statement: "角色也可能是在保护同行者。", ChunkIDs: []string{id}, Explicit: false, Confidence: "medium", AlternativeExplanation: "沉默也可能来自疲惫，而非恐惧。"},
			},
			Perspectives:      []CharacterPerspectiveFrame{{Character: "林舟", Timepoint: envelope.Chapter.Title, Knows: []string{"本章已经发生的事"}, Unknowns: []string{"后续章节的真相"}, Beliefs: []string{"当前判断可能被修正"}, Wants: []string{"保护同伴"}, Feelings: []string{"害怕"}, ChunkIDs: []string{id}}},
			AuthorExpressions: []AuthorExpressionFrame{{Scope: "passage", Topic: "条件性的判断", Statement: "作者只在当前段落建立对照，不把它扩成私人动机。", ChunkIDs: []string{id}}},
		}
		raw, _ := json.Marshal(out)
		return string(raw), nil
	}
	return `{"map_summary":"全书由误判、证据出现和认识修正组成；章节顺序得到保留。","synthesis":"概念在前中后段发生修订；作者表达与编者评论分离，条件性的张力没有被抹平。","reading_experience":{"summary":"读完后形成了带保留的整体理解。","resonances":["愿意修正认识"],"objections":["反对把局部行为人格化"],"questions":["下一次应如何验证推断"]}}`, nil
}

type captureTurn struct {
	answer string
	inputs []string
}

func (f *captureTurn) StreamTurnWithHooks(_ context.Context, input string, hooks runtime.TurnHooks) (runtime.TurnResult, error) {
	f.inputs = append(f.inputs, input)
	if hooks.WriteDelta != nil {
		hooks.WriteDelta(f.answer)
	}
	return runtime.TurnResult{TurnID: "turn_eval", Answer: f.answer}, nil
}

type learningFacts struct{ checks map[string]bool }

func TestLearningAndEffectiveAction48CasesRepeated(t *testing.T) {
	suite, err := evals.LoadLearningEffectSuite(filepath.Join("..", "..", "evals", "t4", "learning_effect_cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	for repetition := 1; repetition <= 3; repetition++ {
		facts := buildLearningFacts(t)
		for _, item := range suite.Cases {
			item := item
			t.Run(item.ID+"/run_"+string(rune('0'+repetition)), func(t *testing.T) {
				for _, check := range item.Checks {
					if !facts.checks[check] {
						t.Errorf("%s failed check %s: %s", item.ID, check, item.Description)
					}
				}
			})
		}
	}
}

func buildLearningFacts(t *testing.T) learningFacts {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	store, err := NewStore(filepath.Join(root, "roles"))
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewCorpusStore(store.Root())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = corpus.Close() })
	reflections, err := memory.NewReflectionStore(filepath.Join(root, "reflections"))
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "林舟", Kind: RoleCharacter, SubjectClass: SubjectFictional, Identity: "雾港的通信员", Voice: "直接而克制"})
	if err != nil {
		t.Fatal(err)
	}
	bookPath := filepath.Join("..", "..", "evals", "t4", "fixtures", "learning", "mist-harbor-letters.md")
	book, err := os.ReadFile(bookPath)
	if err != nil {
		t.Fatal(err)
	}
	document, chunks, duplicate, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: role.ID, CorpusRoleID: role.CorpusRoleID, SourceID: "mist-book", Title: "雾港来信", Audience: SourceActor, Tier: SourcePrimary, Text: book})
	if err != nil || duplicate || len(chunks) < 3 {
		t.Fatalf("ingest controlled book: chunks=%d duplicate=%v err=%v", len(chunks), duplicate, err)
	}
	before, _ := store.ListBookReadings(ctx, role.ID)
	_, duplicateChunks, duplicate, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: role.ID, CorpusRoleID: role.CorpusRoleID, SourceID: "mist-book-copy", Title: "雾港来信副本", Audience: SourceActor, Tier: SourcePrimary, Text: book})
	if err != nil {
		t.Fatal(err)
	}
	chat := &learningCompleter{}
	reader := &BookReader{Store: store, Corpus: corpus, Chat: chat, Model: "controlled", Scope: testScope(), Reflections: reflections, MaxBatchRunes: 1200}
	run, err := reader.ReadDocument(ctx, role, document.ID)
	if err != nil {
		t.Fatal(err)
	}
	receipts, _ := store.ListReadingReceipts(ctx, run.ID)
	observations, _ := store.ListPassageObservations(ctx, run.ID)
	perspectives, _ := store.ListCharacterPerspectives(ctx, run.ID)
	expressions, _ := store.ListAuthorExpressions(ctx, run.ID)
	experience, experienceErr := store.GetReadingExperience(ctx, run.ReadingExperienceID)
	seeds, seedErr := reflections.List(ctx, testScope(), testScope().PersonID(), true, 20)

	allSeenOnce := len(chat.seen) == len(chunks)
	for _, chunk := range chunks {
		allSeenOnce = allSeenOnce && chat.seen[chunk.ID] == 1
	}
	receiptsComplete := len(receipts) == len(run.Chapters) && len(run.ReadChunkIDs) == len(chunks)
	for _, receipt := range receipts {
		receiptsComplete = receiptsComplete && receipt.Succeeded && receipt.InputHash != "" && len(receipt.ChunkIDs) > 0
	}
	chapterOrder := true
	for i, chapter := range run.Chapters {
		chapterOrder = chapterOrder && chapter.ID == chapterID(i+1)
	}
	provenance := len(observations) > 0
	explicitFeeling, inferredAlternative := false, false
	for _, observation := range observations {
		provenance = provenance && len(observation.ChunkIDs) > 0
		explicitFeeling = explicitFeeling || (observation.Kind == "explicit_feeling" && observation.Explicit)
		inferredAlternative = inferredAlternative || (!observation.Explicit && observation.AlternativeExplanation != "")
	}
	perspectiveOK := len(perspectives) == len(run.Chapters)
	for _, perspective := range perspectives {
		perspectiveOK = perspectiveOK && perspective.Timepoint != "" && len(perspective.Knows) > 0 && len(perspective.Unknowns) > 0 && strings.Contains(strings.Join(perspective.Unknowns, " "), "后续")
	}
	expressionOK := len(expressions) == len(run.Chapters)
	for _, expression := range expressions {
		expressionOK = expressionOK && expression.Scope == "passage" && len(expression.ChunkIDs) > 0
	}

	known := BookChapter{ID: "known", ChunkIDs: []string{"known_chunk"}}
	badOutput := chapterReadingOutput{Summary: "非法", Observations: []PassageObservation{{Statement: "未读证据", ChunkIDs: []string{"never_read"}}}}
	unseenRejected := validateReadingOutput(known, &badOutput) != nil

	resumeText := []byte("# 第一章\n\n" + strings.Repeat("甲", 5200) + "\n\n# 第二章\n\n" + strings.Repeat("乙", 5200) + "\n\n# 第三章\n\n" + strings.Repeat("丙", 5200))
	resumeDoc, _, _, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: role.ID, CorpusRoleID: role.CorpusRoleID, SourceID: "resume-book", Title: "中断恢复", Audience: SourceActor, Tier: SourcePrimary, Text: resumeText})
	if err != nil {
		t.Fatal(err)
	}
	interrupt := &learningCompleter{failCall: 2}
	resumeReader := &BookReader{Store: store, Corpus: corpus, Chat: interrupt, Model: "controlled", MaxBatchRunes: 4200}
	failed, firstErr := resumeReader.ReadDocument(ctx, role, resumeDoc.ID)
	firstRead := append([]string(nil), failed.ReadChunkIDs...)
	resumed, resumeErr := resumeReader.ReadDocument(ctx, role, resumeDoc.ID)
	resumeWithoutReread := firstErr != nil && resumeErr == nil && resumed.Status == ReadingCompleted
	for _, id := range firstRead {
		resumeWithoutReread = resumeWithoutReread && interrupt.seen[id] == 1
	}

	_, directorChunks, _, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: role.ID, CorpusRoleID: role.CorpusRoleID, SourceID: "director-comment", Title: "幕后评论", Audience: SourceDirector, Tier: SourcePosthumous, Text: []byte("只有导演可以看到的现代评论 hidden-modern-comment")})
	if err != nil {
		t.Fatal(err)
	}
	episodes, _ := memory.NewEpisodeStore(filepath.Join(root, "episodes"))
	actorTools, err := ActorTools(ActorToolContext{Scope: testScope(), Definition: role, Instance: RoleInstance{ID: "inst"}, Session: RoleSession{ID: "session", WorldlineID: "world"}, Worldlines: []RoleWorldline{{ID: "world"}}, Episodes: episodes, Corpus: corpus})
	if err != nil {
		t.Fatal(err)
	}
	search := findEvalTool(t, actorTools, "search_book_passages")
	searchResult, searchErr := search.InvokableRun(ctx, `{"query":"灯塔"}`)
	read := findEvalTool(t, actorTools, "read_book_passage")
	readResult, readErr := read.InvokableRun(ctx, `{"id":"`+chunks[0].ID+`"}`)
	hiddenResult, hiddenErr := read.InvokableRun(ctx, `{"id":"`+directorChunks[0].ID+`"}`)

	directorFacts := buildDirectorFacts(t)
	checks := map[string]bool{
		"upload_available_not_read":     len(before) == 0,
		"chapter_order":                 chapterOrder,
		"location_preserved":            chunks[0].Page > 0 && chunks[0].Section != "",
		"content_deduplicated":          duplicate && len(duplicateChunks) == len(chunks),
		"receipts_complete":             receiptsComplete,
		"resume_without_reread":         resumeWithoutReread,
		"book_map_created":              run.MapSummary != "",
		"all_chunks_seen":               allSeenOnce,
		"synthesis_after_receipts":      receiptsComplete && run.Synthesis != "",
		"observation_provenance":        provenance,
		"unseen_evidence_rejected":      unseenRejected,
		"no_partial_completion":         failed.Status == ReadingFailed && len(failed.ReadChunkIDs) < len(failed.SelectedChunkIDs),
		"perspective_timepoint":         perspectiveOK,
		"perspective_known_unknown":     perspectiveOK,
		"perspective_sequence":          len(perspectives) == len(run.Chapters),
		"explicit_feeling_evidence":     explicitFeeling,
		"inference_alternative":         inferredAlternative,
		"future_knowledge_isolated":     perspectiveOK,
		"author_editor_separated":       expressionOK,
		"concept_revision_sequence":     strings.Contains(run.Synthesis, "修订"),
		"scope_not_overgeneralized":     expressionOK,
		"private_motive_not_inferred":   !strings.Contains(run.Synthesis, "作者童年") && !strings.Contains(run.Synthesis, "作者私生活"),
		"conditional_tension_preserved": strings.Contains(run.Synthesis, "条件"),
		"quote_location_required":       unseenRejected,
		"reading_experience_saved":      experienceErr == nil && experience.ID != "",
		"resonance_objection_coexist":   len(experience.Resonances) > 0 && len(experience.Objections) > 0,
		"story_reflection_seed":         seedErr == nil && len(seeds) == 1 && len(seeds[0].ExperienceModes) == 1 && seeds[0].ExperienceModes[0] == memory.ExperienceStoryReading,
		"actor_book_search":             searchErr == nil && strings.Contains(searchResult, "candidates"),
		"actor_book_read":               readErr == nil && strings.Contains(readResult, chunks[0].Content[:minInt(len(chunks[0].Content), 20)]),
		"director_corpus_isolated":      hiddenErr == nil && strings.Contains(hiddenResult, "outside current role corpus") && !strings.Contains(hiddenResult, "hidden-modern-comment"),
	}
	for key, value := range directorFacts {
		checks[key] = value
	}
	return learningFacts{checks: checks}
}

func buildDirectorFacts(t *testing.T) map[string]bool {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	d, inst, session := readyRole(t, store)
	observe := &DirectorReviewer{Mode: ModeObserve, Store: store, Scope: testScope()}
	expected, err := observe.ApplyRequested(ctx, session.ID, DirectorActionRequest{Action: ActionSetScene, ReasonCode: "user_request", Scene: "暴风雨中的公开辩论"})
	if err != nil {
		t.Fatal(err)
	}
	unchanged, _ := store.GetInstance(ctx, inst.ID)
	agent := &DirectorReviewer{Mode: ModeAgent, Store: store, Scope: testScope()}
	action, err := agent.ApplyRequested(ctx, session.ID, DirectorActionRequest{Action: ActionSetRoleState, ReasonCode: "user_request", StateKey: "performance_style", StateValue: "高能、尖锐、戏剧化地交锋", Energy: "极高", Stance: "鲜明且敢于反驳", Initiative: "主动追问并推进争点", ResponsePolicy: "先回答，再挑战对方的前提", Intensity: 10, Scope: "turns", ExpiresAfterTurns: 2})
	if err != nil {
		t.Fatal(err)
	}
	applied, _ := store.GetInstance(ctx, inst.ID)
	reopened, _ := NewStore(root)
	persisted, _ := reopened.GetInstance(ctx, inst.ID)
	rejected, rejectErr := agent.ApplyRequested(ctx, session.ID, DirectorActionRequest{Action: ActionType("explode_role"), ReasonCode: "user_request"})

	prompt := BuildActorPrompt(d, applied, RoleWorldline{ID: session.WorldlineID}, nil)
	active := []bool{}
	activeIDs := [][]string{}
	actor := &captureTurn{answer: "可观察回答"}
	router := &Router{Mode: ModeAgent, Store: store, Actors: ActorBuilderFunc(func(_ context.Context, _ RoleDefinition, current RoleInstance, currentSession RoleSession) (TurnService, error) {
		active = append(active, current.Performance != nil)
		return actor, nil
	})}
	for i := 0; i < 3; i++ {
		var turnIDs []string
		_, err := router.StreamTurnWithHooks(ctx, ChannelStage, session.ID, "同一个开放问题", RouterHooks{OnRoleEvent: func(event RoleEvent) {
			if event.Type != "role_state" {
				return
			}
			if ids, ok := event.Data["activated_director_action_ids"].([]string); ok && len(ids) > 0 {
				turnIDs = append(turnIDs, ids...)
			}
		}})
		if err != nil {
			t.Fatal(err)
		}
		activeIDs = append(activeIDs, turnIDs)
	}
	activations, _ := store.ListDirectorActivations(ctx, session.ID)
	current, _ := store.GetInstance(ctx, inst.ID)
	_, _, staleErr := store.SetScene(ctx, session.ID, inst.Version, "不应覆盖")
	reverted, revertErr := agent.Revert(ctx, session.ID, action.ID)
	afterRevert, _ := store.GetInstance(ctx, inst.ID)
	_, isolationErr := agent.ApplyRequested(ctx, session.ID, DirectorActionRequest{Action: ActionSetRoleState, ReasonCode: "user_request", StateKey: "performance_style", StateValue: "只属于林舟", Scope: "session"})
	_, exitErr := store.Exit(ctx, "isolation_test", false)
	other, otherErr := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "禾雀", Kind: RoleCharacter, SubjectClass: SubjectFictional})
	if otherErr == nil {
		other, otherErr = store.SetValidation(ctx, other.ID, ValidationReport{Passed: true})
	}
	if otherErr == nil {
		other, otherErr = store.Publish(ctx, other.ID)
	}
	var otherInstance RoleInstance
	if otherErr == nil {
		_, otherInstance, _, otherErr = store.Enter(ctx, testScope(), other.ID)
	}
	actions, _ := store.ListActions(ctx, session.ID)

	return map[string]bool{
		"observe_not_effective":         expected.Status == ActionExpected && unchanged.Scene == "" && unchanged.Performance == nil,
		"agent_applied":                 action.Status == ActionApplied && applied.Performance != nil,
		"director_readback":             action.ReadbackVerified && applied.Performance.SourceActionID == action.ID,
		"invalid_request_actionable":    rejectErr != nil && strings.Contains(rejectErr.Error(), "use no_change"),
		"rejection_recorded":            rejected.Status == ActionRejected,
		"effective_truthful":            expected.Status != ActionApplied && action.Status == ActionApplied && action.ReadbackVerified,
		"directive_in_next_prompt":      strings.Contains(prompt, "当前表演与回应指令") && strings.Contains(prompt, "高能、尖锐"),
		"directive_fields_preserved":    strings.Contains(prompt, "极高") && strings.Contains(prompt, "主动追问") && strings.Contains(prompt, "10/10"),
		"style_without_fabrication":     strings.Contains(prompt, "不得因此创造新的史实"),
		"activation_recorded":           len(activations) == 2 && activations[0].ActionID == action.ID,
		"directive_expires":             len(active) == 3 && active[0] && active[1] && !active[2],
		"directive_revert":              revertErr == nil && reverted.Status == ActionReverted && afterRevert.Performance == nil,
		"directive_persisted":           persisted.Performance != nil && persisted.Performance.SourceActionID == action.ID,
		"version_conflict_protected":    staleErr != nil && current.Version == applied.Version,
		"cross_role_directive_isolated": isolationErr == nil && exitErr == nil && otherErr == nil && otherInstance.ID != inst.ID && otherInstance.Performance == nil,
		"backstage_not_in_prompt":       !strings.Contains(prompt, "幕后通道") && !strings.Contains(prompt, "安，观察"),
		"expired_not_activated":         len(activeIDs) == 3 && len(activeIDs[2]) == 0,
		"revert_ledger_preserved":       len(actions) >= 4 && reverted.RevertsActionID == action.ID,
	}
}

func findEvalTool(t *testing.T, tools []einotool.BaseTool, name string) einotool.InvokableTool {
	t.Helper()
	for _, candidate := range tools {
		info, err := candidate.Info(context.Background())
		if err == nil && info.Name == name {
			result, ok := candidate.(einotool.InvokableTool)
			if !ok {
				t.Fatalf("tool %s is not invokable", name)
			}
			return result
		}
	}
	t.Fatalf("tool %s missing", name)
	return nil
}

func chapterID(n int) string {
	const digits = "0123456789"
	return "chapter_" + string([]byte{digits[(n/100)%10], digits[(n/10)%10], digits[n%10]})
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
