package theater

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestCharacterArchitectObserveReachesFinalApprovalWithoutMutatingRole(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewCorpusStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer corpus.Close()
	planJSON := `{"target_period":"成熟期","knowledge_cutoff":"故事终章","questions":[{"id":"biography","question":"经历？","priority":"high"}],"required_coverage":["biography"],"completion_criteria":["可追溯"]}`
	architect := &CharacterArchitect{Mode: InitModeObserve, Scope: testScope(), Store: store, Corpus: corpus, Chat: fakeDirectorCompleter{out: planJSON}, CriticChat: fakeDirectorCompleter{out: `{"issues":[]}`}}
	run, role, err := architect.Start(ctx, StartRoleInitializationInput{DisplayName: "林舟", Kind: RoleCharacter, SubjectClass: SubjectLivingPrivate, Objective: "塑造林舟", TargetPeriod: "成熟期", PrivateModelConsent: true})
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.AddSource(ctx, role.ID, "人物小传", "upload", "", "text/plain", []byte("林舟在雾港长大，后来成为修船师。"))
	if err != nil {
		t.Fatal(err)
	}
	_, chunks, _, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: role.ID, CorpusRoleID: role.CorpusRoleID, SourceID: source.ID, Title: source.Title, Audience: SourceActor, Tier: SourceBiography, Text: []byte("林舟在雾港长大，后来成为修船师。")})
	if err != nil || len(chunks) == 0 {
		t.Fatalf("ingest: %v %#v", err, chunks)
	}
	run, err = store.SaveSourceAssessment(ctx, run.ID, SourceAssessment{SourceID: source.ID, Tier: SourceBiography, Audience: SourceActor, Status: AssessmentAccepted, ReadChunkIDs: []string{chunks[0].ID}})
	if err != nil {
		t.Fatal(err)
	}
	blueprintJSON := fmt.Sprintf(`{"target_period":"成熟期","knowledge_cutoff":"故事终章","self_concept":{"key":"self_concept","content":"雾港修船师","chunk_ids":[%q]},"values_and_motives":{"key":"values","content":"重视可靠","chunk_ids":[%q]},"tensions":{"key":"tensions","content":"对离港既向往又迟疑","chunk_ids":[%q]},"reasoning_and_voice":{"key":"voice","content":"简短、具体","chunk_ids":[%q]},"unknown_response_policy":{"key":"unknown","content":"不知道时承认未知"},"allowed_inferences":{"key":"inferences","content":"只允许局部推断"},"forbidden_anachronisms":{"key":"forbidden_anachronisms","content":"不得使用故事终章后的知识"}}`, chunks[0].ID, chunks[0].ID, chunks[0].ID, chunks[0].ID)
	architect.Chat = fakeDirectorCompleter{out: blueprintJSON}
	run, err = store.ApproveResearchPlan(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	run, err = architect.Continue(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != InitAwaitingFinalApproval {
		t.Fatalf("status=%s err=%s", run.Status, run.ErrorSummary)
	}
	unchanged, _ := store.GetDefinition(ctx, role.ID)
	if unchanged.Status != DefinitionDraft || unchanged.Identity != "" {
		t.Fatalf("observe mutated definition: %#v", unchanged)
	}
	if len(run.Events) == 0 {
		t.Fatal("public initialization trace missing")
	}
}

func TestCharacterArchitectPrivateRoleRequiresModelConsent(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	corpus, _ := NewCorpusStore(store.Root())
	defer corpus.Close()
	architect := &CharacterArchitect{Mode: InitModeObserve, Scope: testScope(), Store: store, Corpus: corpus, Chat: fakeDirectorCompleter{out: `{}`}}
	_, _, err := architect.Start(context.Background(), StartRoleInitializationInput{DisplayName: "私人角色", Kind: RoleCharacter, SubjectClass: SubjectLivingPrivate, Objective: "测试"})
	if err == nil {
		t.Fatal("private role planning should require consent")
	}
}

func TestDecodeBlueprintAcceptsRelationshipObject(t *testing.T) {
	raw := `{"target_period":"成熟期","knowledge_cutoff":"1937","self_concept":{"content":"自己"},"values_and_motives":{"content":"价值"},"tensions":{"content":"张力"},"relationships":{"key":"relationships","content":"与同事的关系"},"reasoning_and_voice":{"content":"声音"},"unknown_response_policy":{"content":"承认未知"},"allowed_inferences":{"content":"有限推断"},"forbidden_anachronisms":{"content":"禁止越界"}}`
	var value RoleBlueprint
	if err := decodeBlueprintJSONObject(raw, &value); err != nil {
		t.Fatal(err)
	}
	if len(value.Relationships) != 1 || value.Relationships[0].Content != "与同事的关系" {
		t.Fatalf("relationship object was not normalized: %#v", value.Relationships)
	}
	encoded, err := json.Marshal(value)
	if err != nil || !strings.Contains(string(encoded), `"relationships":[`) {
		t.Fatalf("relationships must marshal as an array: %s %v", encoded, err)
	}
}

func TestDecodeBlueprintNormalizesTextLikeMetadata(t *testing.T) {
	raw := `{"target_period":{"start":"1912","end":"1937","description":"成熟期"},"knowledge_cutoff":["1937-05-28","不得使用后世知识"],"change_summary":{"summary":"首次塑造"},"self_concept":{"content":"自己"},"values_and_motives":{"content":"价值"},"tensions":{"content":"张力"},"relationships":[],"reasoning_and_voice":{"content":"声音"},"unknown_response_policy":{"content":"承认未知"},"allowed_inferences":{"content":"有限推断"},"forbidden_anachronisms":{"content":"禁止越界"}}`
	var value RoleBlueprint
	if err := decodeBlueprintJSONObject(raw, &value); err != nil {
		t.Fatal(err)
	}
	if value.TargetPeriod != "成熟期 — 1912 — 1937" {
		t.Fatalf("target period = %q", value.TargetPeriod)
	}
	if value.KnowledgeCutoff != "1937-05-28；不得使用后世知识" {
		t.Fatalf("knowledge cutoff = %q", value.KnowledgeCutoff)
	}
	if value.ChangeSummary != "首次塑造" {
		t.Fatalf("change summary = %q", value.ChangeSummary)
	}
}

func TestSelectBlueprintEvidenceBalancesSourcesAndBoundsPayload(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewCorpusStore(store.Root())
	if err != nil {
		t.Fatal(err)
	}
	defer corpus.Close()
	role, err := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "研究人物", Kind: RoleCharacter, SubjectClass: SubjectFictional})
	if err != nil {
		t.Fatal(err)
	}
	_, actorChunks, _, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: role.ID, CorpusRoleID: role.CorpusRoleID, SourceID: "source-actor", Title: "Primary alpha", Audience: SourceActor, Tier: SourcePrimary, Text: []byte(strings.Repeat("alpha theory voice evidence.\n\n", 700))})
	if err != nil {
		t.Fatal(err)
	}
	_, directorChunks, _, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: role.ID, CorpusRoleID: role.CorpusRoleID, SourceID: "source-director", Title: "Biography beta", Audience: SourceDirector, Tier: SourceBiography, Text: []byte(strings.Repeat("beta biography relationship evidence.\n\n", 500))})
	if err != nil {
		t.Fatal(err)
	}
	chunkIDs := func(chunks []RoleChunk) []string {
		ids := make([]string, 0, len(chunks))
		for _, chunk := range chunks {
			ids = append(ids, chunk.ID)
		}
		return ids
	}
	run := RoleInitializationRun{Plan: &RoleResearchPlan{Questions: []ResearchQuestion{{Question: "alpha beta", Topics: []string{"theory", "biography"}}}}, Assessments: []SourceAssessment{
		{SourceID: "source-actor", Tier: SourcePrimary, Audience: SourceActor, Status: AssessmentAccepted, ReadChunkIDs: chunkIDs(actorChunks)},
		{SourceID: "source-director", Tier: SourceBiography, Audience: SourceDirector, Status: AssessmentAccepted, ReadChunkIDs: chunkIDs(directorChunks)},
	}}
	architect := &CharacterArchitect{Corpus: corpus}
	got := architect.selectBlueprintEvidence(ctx, run, role)
	if len(got) == 0 || len(got) > maxBlueprintEvidenceChunks {
		t.Fatalf("evidence count = %d", len(got))
	}
	sources := map[string]bool{}
	for _, chunk := range got {
		sources[chunk.SourceID] = true
		if len([]rune(chunk.Content)) > maxBlueprintChunkRunes+1 {
			t.Fatalf("chunk %s exceeds payload bound", chunk.ID)
		}
	}
	if !sources["source-actor"] || !sources["source-director"] {
		t.Fatalf("evidence did not preserve source diversity: %#v", sources)
	}
}

func TestSelectCriticEvidenceIncludesCitedBodies(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewCorpusStore(store.Root())
	if err != nil {
		t.Fatal(err)
	}
	defer corpus.Close()
	role, _ := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "证据人物", Kind: RoleCharacter, SubjectClass: SubjectFictional})
	_, chunks, _, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: role.ID, CorpusRoleID: role.CorpusRoleID, SourceID: "source", Title: "Evidence", Audience: SourceActor, Tier: SourcePrimary, Text: []byte(strings.Repeat("cited evidence body ", 400))})
	if err != nil || len(chunks) < 2 {
		t.Fatalf("ingest chunks=%d err=%v", len(chunks), err)
	}
	blueprint := RoleBlueprint{SelfConcept: BlueprintSection{Key: "self_concept", ChunkIDs: []string{chunks[0].ID, chunks[1].ID, chunks[0].ID}}}
	got := (&CharacterArchitect{Corpus: corpus}).selectCriticEvidence(ctx, blueprint)
	if len(got) != 2 {
		t.Fatalf("critic evidence count = %d", len(got))
	}
	for _, chunk := range got {
		if cleanText(chunk.Content) == "" || len([]rune(chunk.Content)) > maxCriticChunkRunes+1 {
			t.Fatalf("critic evidence body invalid: %#v", chunk)
		}
	}
}

func TestCharacterArchitectPausesBeforeBlueprintWithoutReadEvidence(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewCorpusStore(store.Root())
	if err != nil {
		t.Fatal(err)
	}
	defer corpus.Close()
	role, _ := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "无资料人物", Kind: RoleCharacter, SubjectClass: SubjectFictional})
	run, _ := store.CreateInitialization(ctx, role.ID, "测试证据门", false, "fixture")
	run, _ = store.SaveResearchPlan(ctx, run.ID, RoleResearchPlan{TargetPeriod: "成熟期", Questions: []ResearchQuestion{{ID: "biography", Question: "经历？"}}})
	run, _ = store.ApproveResearchPlan(ctx, run.ID)
	run, _ = store.TransitionInitialization(ctx, run.ID, InitAnalyzing, "coverage", "sources_collected", "")
	run, _ = store.TransitionInitialization(ctx, run.ID, InitCompiling, "compile", "coverage_analyzed", "")
	run, _ = store.TransitionInitialization(ctx, run.ID, InitBlueprinting, "blueprint", "compiled", "")
	architect := &CharacterArchitect{Mode: InitModeObserve, Scope: testScope(), Store: store, Corpus: corpus}
	run, err = architect.Continue(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != InitPaused || run.ResumeStatus != InitAnalyzing || run.Checkpoint != "evidence_required" {
		t.Fatalf("expected evidence pause, got %#v", run)
	}
}

func TestBlueprintGeneratedEvidenceIsHardError(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, _ := NewStore(root)
	corpus, _ := NewCorpusStore(root)
	defer corpus.Close()
	role, _ := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "梦中人", Kind: RoleCharacter, SubjectClass: SubjectFictional})
	run, _ := store.CreateInitialization(ctx, role.ID, "test", true, "fixture")
	source, _ := store.AddSource(ctx, role.ID, "生成假设", "generated", "", "text/plain", []byte("这是生成内容"))
	_, chunks, _, _ := corpus.Ingest(ctx, CorpusIngestInput{RoleID: role.ID, CorpusRoleID: role.CorpusRoleID, SourceID: source.ID, Title: source.Title, Audience: SourceActor, Tier: SourceGenerated, Text: []byte("这是生成内容")})
	run, _ = store.SaveSourceAssessment(ctx, run.ID, SourceAssessment{SourceID: source.ID, Tier: SourceGenerated, Audience: SourceActor, Status: AssessmentAccepted, ReadChunkIDs: []string{chunks[0].ID}})
	blueprint := RoleBlueprint{TargetPeriod: "虚构时期", KnowledgeCutoff: "终章", SelfConcept: BlueprintSection{Key: "self_concept", Content: "生成身份", ChunkIDs: []string{chunks[0].ID}}}
	issues := ValidateBlueprintEvidence(ctx, store, corpus, run, blueprint)
	if !hasUnresolvedHardIssue(issues) {
		t.Fatalf("generated evidence should be hard: %#v", issues)
	}
}

func TestCharacterArchitectPeriodVariantSharesCorpusNotBlueprint(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, _ := NewStore(root)
	corpus, _ := NewCorpusStore(root)
	defer corpus.Close()
	parent, _ := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "林舟·学徒期", Kind: RoleCharacter, SubjectClass: SubjectFictional, TargetPeriod: "学徒期"})
	architect := &CharacterArchitect{Mode: InitModeObserve, Scope: testScope(), Store: store, Corpus: corpus, Chat: fakeDirectorCompleter{out: `{"target_period":"成熟期","questions":[{"id":"ideas","question":"思想变化？"}]}`}}
	run, variant, err := architect.Start(ctx, StartRoleInitializationInput{DisplayName: "林舟·成熟期", Kind: RoleCharacter, SubjectClass: SubjectFictional, TargetPeriod: "成熟期", VariantOfRoleID: parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	if variant.ID == parent.ID || variant.CorpusRoleID != parent.CorpusRoleID || run.VariantOfRoleID != parent.ID {
		t.Fatalf("variant isolation/share failed parent=%#v variant=%#v run=%#v", parent, variant, run)
	}
	if variant.BlueprintVersion != 0 || variant.InitializationRunID != "" {
		t.Fatal("new period variant inherited blueprint state")
	}
}

func TestDecodeBlueprintRejectsEmptyAndAcceptsWrapper(t *testing.T) {
	var value RoleBlueprint
	if err := decodeBlueprintJSONObject(`{"blueprint":{"self_concept":{"content":"自己"},"values_and_motives":{"content":"价值"},"reasoning_and_voice":{"content":"声音"},"unknown_response_policy":{"content":"承认未知"},"allowed_inferences":{"content":"有限推断"},"forbidden_anachronisms":{"content":"禁止越界"}}}`, &value); err != nil {
		t.Fatal(err)
	}
	if value.SelfConcept.Content != "自己" {
		t.Fatalf("wrapper not decoded: %#v", value)
	}
	if err := decodeBlueprintJSONObject(`{"target_period":"x"}`, &value); err == nil {
		t.Fatal("empty blueprint should fail parsing")
	}
}

type recordingArchitectCompleter struct {
	out   string
	input string
}

func (f *recordingArchitectCompleter) Complete(_ context.Context, _ string, input string) (string, error) {
	f.input = input
	return f.out, nil
}

func TestBuildBlueprintIncludesRevisionContext(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewCorpusStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer corpus.Close()
	role, err := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "修订人物", Kind: RoleCharacter, SubjectClass: SubjectFictional})
	if err != nil {
		t.Fatal(err)
	}
	run, err := store.CreateInitialization(ctx, role.ID, "验证修订闭环", false, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.AddSource(ctx, role.ID, "人物资料", "upload", "", "text/plain", []byte("人物只相信可以核验的经历。"))
	if err != nil {
		t.Fatal(err)
	}
	_, chunks, _, err := corpus.Ingest(ctx, CorpusIngestInput{RoleID: role.ID, CorpusRoleID: role.CorpusRoleID, SourceID: source.ID, Title: source.Title, Audience: SourceActor, Tier: SourcePrimary, Text: []byte("人物只相信可以核验的经历。")})
	if err != nil || len(chunks) == 0 {
		t.Fatalf("ingest: %v", err)
	}
	run, err = store.SaveSourceAssessment(ctx, run.ID, SourceAssessment{SourceID: source.ID, Tier: SourcePrimary, Audience: SourceActor, Status: AssessmentAccepted, ReadChunkIDs: []string{chunks[0].ID}})
	if err != nil {
		t.Fatal(err)
	}
	previous, run, err := store.SaveBlueprint(ctx, RoleBlueprint{RunID: run.ID, RoleID: role.ID, Version: 1, TargetPeriod: "旧时期", KnowledgeCutoff: "旧截止", SelfConcept: BlueprintSection{Key: "self_concept", Content: "旧画像", ChunkIDs: []string{chunks[0].ID}}})
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = store.SaveCritique(ctx, RoleCritique{RunID: run.ID, BlueprintID: previous.ID, Issues: []CritiqueIssue{{Code: "UNSUPPORTED_DETAIL", Severity: CritiqueHard, Message: "删除无证据细节"}}})
	if err != nil {
		t.Fatal(err)
	}
	run.RevisionRequest = "只保留逐项有证据的内容"
	output := fmt.Sprintf(`{"target_period":"新时期","knowledge_cutoff":"新截止","self_concept":{"key":"self_concept","content":"核验后的画像","chunk_ids":[%q]},"values_and_motives":{"key":"values","content":"重视核验","chunk_ids":[%q]},"tensions":{"key":"tensions","content":""},"relationships":[],"reasoning_and_voice":{"key":"voice","content":"不依据当前资料建立固定语言风格"},"unknown_response_policy":{"key":"unknown","content":"证据不足时承认未知"},"allowed_inferences":{"key":"inferences","content":"仅作有限推断"},"forbidden_anachronisms":{"key":"forbidden_anachronisms","content":"不得越过知识截止"}}`, chunks[0].ID, chunks[0].ID)
	chat := &recordingArchitectCompleter{out: output}
	architect := &CharacterArchitect{Store: store, Corpus: corpus, Chat: chat}
	if _, err := architect.buildBlueprint(ctx, run, role); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"revision_request", "只保留逐项有证据的内容", "previous_blueprint", "旧画像", "previous_critique", "UNSUPPORTED_DETAIL"} {
		if !strings.Contains(chat.input, want) {
			t.Fatalf("architect input missing %q: %s", want, chat.input)
		}
	}
}

func TestCritiqueCoverageErrorsReturnToAnalysis(t *testing.T) {
	coverageHard := RoleCritique{Issues: []CritiqueIssue{{Code: "UNSUPPORTED_SOURCE_INDEPENDENCE", Severity: CritiqueHard, Section: "coverage.biography"}}}
	if !critiqueRequiresCoverageRefresh(coverageHard) {
		t.Fatal("coverage hard error must return to analysis")
	}
	if !validInitializationTransition(InitCritiquing, InitAnalyzing) {
		t.Fatal("critic must be allowed to return to coverage analysis")
	}
	coverageWarning := RoleCritique{Issues: []CritiqueIssue{{Code: "STALE_COVERAGE_MATRIX", Severity: CritiqueWarning, Section: "coverage"}}}
	if critiqueRequiresCoverageRefresh(coverageWarning) {
		t.Fatal("coverage warning must not block final review")
	}
	blueprintHard := RoleCritique{Issues: []CritiqueIssue{{Code: "UNSOURCED_PERIODIZATION", Severity: CritiqueHard, Section: "blueprint.target_period"}}}
	if critiqueRequiresCoverageRefresh(blueprintHard) {
		t.Fatal("blueprint-only error must stay in blueprint revision")
	}
}

type sequenceArchitectCompleter struct {
	outputs []string
	calls   int
}

func (f *sequenceArchitectCompleter) Complete(_ context.Context, _ string, _ string) (string, error) {
	index := f.calls
	f.calls++
	if index >= len(f.outputs) {
		index = len(f.outputs) - 1
	}
	return f.outputs[index], nil
}

func TestCoverageAnalysisRetriesInvalidJSON(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := NewCorpusStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer corpus.Close()
	role, err := store.CreateDefinition(ctx, testScope(), RoleDefinitionWrite{DisplayName: "覆盖人物", Kind: RoleCharacter, SubjectClass: SubjectFictional})
	if err != nil {
		t.Fatal(err)
	}
	run, err := store.CreateInitialization(ctx, role.ID, "验证覆盖重试", false, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	run, err = store.updateInitialization(ctx, run.ID, 0, func(value *RoleInitializationRun) error {
		value.Status = InitAnalyzing
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	chat := &sequenceArchitectCompleter{outputs: []string{"not json", `{"coverage":{"items":[{"dimension":"biography","state":"missing","summary":"证据不足"}]},"conflicts":[]}`}}
	architect := &CharacterArchitect{Store: store, Corpus: corpus, CoverageChat: chat}
	updated, err := architect.analyze(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if chat.calls != 2 {
		t.Fatalf("coverage calls = %d", chat.calls)
	}
	if updated.Status != InitCompiling || len(updated.Coverage.Items) != 1 || updated.Coverage.Items[0].Summary != "证据不足" {
		t.Fatalf("unexpected coverage retry result: %#v", updated)
	}
}
