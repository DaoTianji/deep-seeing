package memory

// Retrieval is a derived index, never the authority for Episode contents or access.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"deep-seeing/internal/identity"
)

type RetrievalHit struct {
	Episode Episode
	Preview string
}

type RetrievalResult struct {
	Hits     []RetrievalHit
	Backend  string
	Fallback string
}

type EpisodeRetriever struct {
	Store     *EpisodeStore
	Scope     identity.TenantScope
	Backend   string
	Hindsight *HindsightClient
}

func ParseRetrievalBackend(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "legacy":
		return "legacy", true
	case "bm25":
		return "bm25", true
	case "hindsight":
		return "hindsight", true
	default:
		return "legacy", false
	}
}

// Snapshot scans the factual files, not the bounded recent-index Markdown.
func (s *EpisodeStore) Snapshot(ctx context.Context) ([]Episode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.byIDDir())
	if err != nil {
		return nil, err
	}
	var out []Episode
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// macOS archive transfers can contain AppleDouble sidecars (._*.md).
		// These are filesystem metadata, never Episode documents.
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		ep, err := s.readEpisodeLocked(id)
		if err != nil {
			return nil, fmt.Errorf("episode snapshot: unreadable source")
		}
		out = append(out, ep)
	}
	return out, nil
}

// Ordinary recall excludes roles, including unassigned historical roleplay.
func OrdinaryEpisodeAllowed(ep Episode, scope identity.TenantScope) bool {
	if !IsActiveEpisode(ep) || isRoleEpisode(ep) {
		return false
	}
	return len(ep.PersonIDs) == 0 || stringIn(scope.PersonID(), ep.PersonIDs)
}

func EpisodeRevision(ep Episode) string {
	b, _ := json.Marshal(ep)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (r *EpisodeRetriever) Search(ctx context.Context, scope identity.TenantScope, query string, limit int) (RetrievalResult, error) {
	result := RetrievalResult{Backend: r.Backend, Hits: []RetrievalHit{}}
	if err := scope.Validate(); err != nil {
		return result, err
	}
	if scope != r.Scope {
		return result, fmt.Errorf("retrieval scope denied")
	}
	if limit <= 0 {
		limit = 8
	}
	if limit > 50 {
		limit = 50
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return result, nil
	}
	if len([]rune(query)) > 2000 {
		return result, fmt.Errorf("recall query exceeds 2000 characters")
	}
	if r.Backend == "legacy" {
		eps, err := r.Store.Search(ctx, scope, Query{Text: query, Limit: limit})
		for _, ep := range eps {
			if OrdinaryEpisodeAllowed(ep, scope) {
				result.Hits = append(result.Hits, RetrievalHit{Episode: ep})
			}
		}
		return result, err
	}
	if r.Backend == "hindsight" {
		if r.Hindsight == nil {
			return result, fmt.Errorf("hindsight is not configured")
		}
		facts, err := r.Hindsight.Recall(ctx, scope, query)
		if err == nil {
			seen := map[string]bool{}
			for _, fact := range facts {
				id := fact.DocumentID
				if id == "" || fact.Metadata["episode_id"] != id || seen[id] {
					continue
				}
				// Never trust the remote index for body, lifecycle, ownership or revision.
				ep, err := r.Store.Get(ctx, id)
				if err != nil || ep.ID != id || !OrdinaryEpisodeAllowed(ep, scope) || fact.Metadata["revision"] != EpisodeRevision(ep) {
					continue
				}
				seen[id] = true
				result.Hits = append(result.Hits, RetrievalHit{Episode: ep, Preview: QueryPreview(ep.Content, query)})
				if len(result.Hits) == limit {
					break
				}
			}
			// A successful empty result stays empty. Never substitute recent episodes.
			return result, nil
		}
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		result.Backend, result.Fallback = "bm25", "hindsight_unavailable"
	}
	eps, err := r.Store.Snapshot(ctx)
	if err != nil {
		return result, err
	}
	var eligible []Episode
	for _, ep := range eps {
		if OrdinaryEpisodeAllowed(ep, scope) {
			eligible = append(eligible, ep)
		}
	}
	for _, ep := range rankEpisodes(eligible, query, limit) {
		result.Hits = append(result.Hits, RetrievalHit{Episode: ep, Preview: QueryPreview(ep.Content, query)})
	}
	return result, nil
}

// Latin terms are words; Han runs use overlapping bigrams (not arbitrary
// English character pairs). No model calls are needed by the lexical fallback.
func retrievalTerms(text string) []string {
	var out []string
	var word, han []rune
	flushWord := func() {
		if len(word) > 0 {
			out = append(out, string(word))
			word = nil
		}
	}
	flushHan := func() {
		if len(han) == 1 {
			out = append(out, string(han))
		}
		for i := 0; i+1 < len(han); i++ {
			out = append(out, string(han[i:i+2]))
		}
		han = nil
	}
	for _, ch := range strings.ToLower(text) {
		if unicode.Is(unicode.Han, ch) {
			flushWord()
			han = append(han, ch)
		} else {
			flushHan()
			if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
				word = append(word, ch)
			} else {
				flushWord()
			}
		}
	}
	flushWord()
	flushHan()
	return out
}

func rankEpisodes(eps []Episode, query string, limit int) []Episode {
	tfs := make([]map[string]int, len(eps))
	lengths := make([]int, len(eps))
	df := map[string]int{}
	average := 0.0
	for i, ep := range eps {
		tf := map[string]int{}
		terms := retrievalTerms(ep.Content + " " + ep.Why)
		for _, term := range terms {
			tf[term]++
		}
		for term := range tf {
			df[term]++
		}
		tfs[i] = tf
		lengths[i] = len(terms)
		average += float64(len(terms))
	}
	if len(eps) == 0 || average == 0 {
		return nil
	}
	average /= float64(len(eps))
	unique := map[string]bool{}
	for _, term := range retrievalTerms(query) {
		unique[term] = true
	}
	terms := make([]string, 0, len(unique))
	for term := range unique {
		terms = append(terms, term)
	}
	sort.Strings(terms)
	type ranked struct {
		index int
		score float64
	}
	var ranks []ranked
	for i := range eps {
		score := 0.0
		for _, term := range terms {
			freq := float64(tfs[i][term])
			if freq == 0 {
				continue
			}
			idf := math.Log(1 + (float64(len(eps)-df[term])+0.5)/(float64(df[term])+0.5))
			score += idf * freq * 2.2 / (freq + 1.2*(0.25+0.75*float64(lengths[i])/average))
		}
		if score > 0 {
			ranks = append(ranks, ranked{i, score})
		}
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].score == ranks[j].score {
			return eps[ranks[i].index].ID < eps[ranks[j].index].ID
		}
		return ranks[i].score > ranks[j].score
	})
	if len(ranks) > limit {
		ranks = ranks[:limit]
	}
	out := make([]Episode, 0, len(ranks))
	for _, rank := range ranks {
		out = append(out, eps[rank.index])
	}
	return out
}

// Preview is an exact bounded excerpt from the authoritative body, not a
// generated fact. Returning it never counts as an explicit read or adoption.
func QueryPreview(content, query string) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) == 0 {
		return ""
	}
	lower := strings.ToLower(string(runes))
	match := -1
	for _, term := range retrievalTerms(query) {
		if pos := strings.Index(lower, term); pos >= 0 && (match < 0 || pos < match) {
			match = pos
		}
	}
	start := 0
	if match >= 0 {
		start = len([]rune(lower[:match])) - 40
	}
	if start < 0 {
		start = 0
	}
	size := 160
	if len(runes) <= size {
		size = len(runes) * 3 / 4
		if size < 1 {
			return "[read required]"
		}
	}
	end := start + size
	if end > len(runes) {
		end = len(runes)
	}
	preview := string(runes[start:end])
	if start > 0 {
		preview = "…" + preview
	}
	if end < len(runes) {
		preview += "…"
	}
	return preview
}
