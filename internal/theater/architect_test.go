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
