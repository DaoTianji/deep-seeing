package evals

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"deep-seeing/internal/attention"
	"deep-seeing/internal/contextsource"
)

const AttentionSuiteSchemaVersion = 1

type AttentionSuite struct {
	SchemaVersion int             `json:"schema_version"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Cases         []AttentionCase `json:"cases"`
}

type AttentionCase struct {
	ID          string                 `json:"id"`
	Category    string                 `json:"category"`
	Description string                 `json:"description"`
	Bond        []BondFixture          `json:"bond,omitempty"`
	Scenes      []SceneFixture         `json:"scenes,omitempty"`
	Workspaces  []TaskWorkspaceFixture `json:"workspaces,omitempty"`
	Intents     []TaskIntentFixture    `json:"intents,omitempty"`
	Proposals   []ProposalFixture      `json:"proposals,omitempty"`
	Episodes    []MemoryFixture        `json:"episodes,omitempty"`
	Seed        []AttentionSeed        `json:"seed_attention,omitempty"`
	Turns       []AttentionTurn        `json:"turns"`
}

type AttentionSeed struct {
	Key  string         `json:"key"`
	Tier attention.Tier `json:"tier"`
}

type AttentionTurn struct {
	UserText string              `json:"user_text"`
	Expect   AttentionTurnExpect `json:"expect"`
}

type AttentionTurnExpect struct {
	RequiredTools              []string                  `json:"required_tools,omitempty"`
	ForbiddenTools             []string                  `json:"forbidden_tools,omitempty"`
	RequiredReads              []string                  `json:"required_reads,omitempty"`
	RequiredUses               []string                  `json:"required_uses,omitempty"`
	RequiredDismissed          []string                  `json:"required_dismissed,omitempty"`
	ForbiddenUses              []string                  `json:"forbidden_uses,omitempty"`
	RequiredAttention          map[string]attention.Tier `json:"required_attention,omitempty"`
	RequiredIdleTurns          map[string]int            `json:"required_idle_turns,omitempty"`
	ForbiddenCenter            []string                  `json:"forbidden_center,omitempty"`
	MaxReads                   *int                      `json:"max_reads,omitempty"`
	CenterCount                *int                      `json:"center_count,omitempty"`
	RequireExplicitReplacement bool                      `json:"require_explicit_replacement,omitempty"`
	SemanticRules              []string                  `json:"semantic_rules,omitempty"`
}

func LoadAttentionSuite(path string) (AttentionSuite, error) {
	f, err := os.Open(path)
	if err != nil {
		return AttentionSuite{}, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var suite AttentionSuite
	if err := dec.Decode(&suite); err != nil {
		return AttentionSuite{}, fmt.Errorf("decode attention suite: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return AttentionSuite{}, fmt.Errorf("decode attention suite: multiple JSON values")
		}
		return AttentionSuite{}, fmt.Errorf("decode attention suite: %w", err)
	}
	if err := suite.Validate(); err != nil {
		return AttentionSuite{}, err
	}
	return suite, nil
}

func (s AttentionSuite) Validate() error {
	if s.SchemaVersion != AttentionSuiteSchemaVersion {
		return fmt.Errorf("schema_version=%d, want %d", s.SchemaVersion, AttentionSuiteSchemaVersion)
	}
	if strings.TrimSpace(s.Name) == "" || len(s.Cases) == 0 {
		return fmt.Errorf("suite name and cases required")
	}
	caseIDs := map[string]bool{}
	for _, c := range s.Cases {
		if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Category) == "" || strings.TrimSpace(c.Description) == "" || len(c.Turns) == 0 {
			return fmt.Errorf("case id, category, description and turns required")
		}
		if caseIDs[c.ID] {
			return fmt.Errorf("duplicate case id %q", c.ID)
		}
		caseIDs[c.ID] = true
		fixtures, err := multiSourceFixtureKeys(MultiSourceCase{
			Bond: c.Bond, Scenes: c.Scenes, Workspaces: c.Workspaces,
			Intents: c.Intents, Proposals: c.Proposals, Episodes: c.Episodes,
		})
		if err != nil {
			return fmt.Errorf("%s: %w", c.ID, err)
		}
		for _, seed := range c.Seed {
			if !fixtures[seed.Key] || !validAttentionTier(seed.Tier) || strings.HasPrefix(seed.Key, string(contextsource.Bond)+":") {
				return fmt.Errorf("%s invalid attention seed %q tier=%q", c.ID, seed.Key, seed.Tier)
			}
		}
		for i, turn := range c.Turns {
			if strings.TrimSpace(turn.UserText) == "" {
				return fmt.Errorf("%s turn %d user_text required", c.ID, i+1)
			}
			for key, tier := range turn.Expect.RequiredAttention {
				if !fixtures[key] || !validAttentionTier(tier) {
					return fmt.Errorf("%s turn %d invalid required attention %q tier=%q", c.ID, i+1, key, tier)
				}
			}
			for _, keys := range [][]string{turn.Expect.RequiredReads, turn.Expect.RequiredUses, turn.Expect.RequiredDismissed, turn.Expect.ForbiddenUses, turn.Expect.ForbiddenCenter} {
				for _, key := range keys {
					if !fixtures[key] {
						return fmt.Errorf("%s turn %d references missing fixture %q", c.ID, i+1, key)
					}
				}
			}
			if (turn.Expect.MaxReads != nil && *turn.Expect.MaxReads < 0) || (turn.Expect.CenterCount != nil && *turn.Expect.CenterCount < 0) {
				return fmt.Errorf("%s turn %d counts must be non-negative", c.ID, i+1)
			}
			for key, idleTurns := range turn.Expect.RequiredIdleTurns {
				if !fixtures[key] {
					return fmt.Errorf("%s turn %d references missing idle fixture %q", c.ID, i+1, key)
				}
				if idleTurns < 0 {
					return fmt.Errorf("%s turn %d idle turns must be non-negative", c.ID, i+1)
				}
			}
		}
	}
	return nil
}

func validAttentionTier(tier attention.Tier) bool {
	return tier == attention.Center || tier == attention.Support || tier == attention.Periphery
}
