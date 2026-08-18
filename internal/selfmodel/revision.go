package selfmodel

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"deep-seeing/internal/memory"
)

func formatArtifact(a Artifact) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", a.ID)
	fmt.Fprintf(&b, "type: %s\n", a.Type)
	fmt.Fprintf(&b, "status: %s\n", a.Status)
	fmt.Fprintf(&b, "title: %q\n", a.Title)
	fmt.Fprintf(&b, "summary: %q\n", a.Summary)
	fmt.Fprintf(&b, "confidence: %.3f\n", a.Confidence)
	fmt.Fprintf(&b, "created_at: %s\n", a.CreatedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "updated_at: %s\n", a.UpdatedAt.UTC().Format(time.RFC3339))
	if len(a.SourceEpisodeIDs) > 0 {
		fmt.Fprintf(&b, "source_episode_ids: [%s]\n", strings.Join(quoteAll(a.SourceEpisodeIDs), ", "))
	}
	if len(a.ExperienceModes) > 0 {
		modes := make([]string, len(a.ExperienceModes))
		for i, m := range a.ExperienceModes {
			modes[i] = string(m)
		}
		fmt.Fprintf(&b, "experience_modes: [%s]\n", strings.Join(quoteAll(modes), ", "))
	}
	b.WriteString("---\n\n")
	b.WriteString("## Body\n\n")
	b.WriteString(strings.TrimSpace(a.Body))
	b.WriteString("\n\n## Revisions\n\n")
	for _, r := range a.Revisions {
		fmt.Fprintf(&b, "- %s | %s | %s\n", r.At.UTC().Format(time.RFC3339), r.Actor, r.Summary)
	}
	return b.String()
}

func parseArtifact(fallbackID, raw string) (Artifact, error) {
	a := Artifact{ID: fallbackID, Type: TypePattern, Status: StatusObservation}
	body := raw
	if strings.HasPrefix(raw, "---\n") {
		parts := strings.SplitN(raw, "---\n", 3)
		if len(parts) >= 3 {
			for _, line := range strings.Split(parts[1], "\n") {
				line = strings.TrimSpace(line)
				if after, ok := strings.CutPrefix(line, "id:"); ok {
					a.ID = strings.TrimSpace(after)
				}
				if after, ok := strings.CutPrefix(line, "type:"); ok {
					a.Type = NormalizeType(after)
				}
				if after, ok := strings.CutPrefix(line, "status:"); ok {
					a.Status = NormalizeStatus(after)
				}
				if after, ok := strings.CutPrefix(line, "title:"); ok {
					a.Title = strings.Trim(strings.TrimSpace(after), `"`)
				}
				if after, ok := strings.CutPrefix(line, "summary:"); ok {
					a.Summary = strings.Trim(strings.TrimSpace(after), `"`)
				}
				if after, ok := strings.CutPrefix(line, "confidence:"); ok {
					if f, err := strconv.ParseFloat(strings.TrimSpace(after), 64); err == nil {
						a.Confidence = f
					}
				}
				if after, ok := strings.CutPrefix(line, "source_episode_ids:"); ok {
					a.SourceEpisodeIDs = parseList(after)
				}
				if after, ok := strings.CutPrefix(line, "experience_modes:"); ok {
					for _, m := range parseList(after) {
						a.ExperienceModes = append(a.ExperienceModes, memory.NormalizeExperienceMode(m))
					}
				}
				if after, ok := strings.CutPrefix(line, "created_at:"); ok {
					if t, err := time.Parse(time.RFC3339, strings.TrimSpace(after)); err == nil {
						a.CreatedAt = t
					}
				}
				if after, ok := strings.CutPrefix(line, "updated_at:"); ok {
					if t, err := time.Parse(time.RFC3339, strings.TrimSpace(after)); err == nil {
						a.UpdatedAt = t
					}
				}
			}
			body = parts[2]
		}
	}
	a.Body, a.Revisions = splitBodyRevisions(body)
	if a.ID == "" {
		a.ID = fallbackID
	}
	return a, nil
}

func splitBodyRevisions(raw string) (string, []Revision) {
	const marker = "## Revisions"
	idx := strings.Index(raw, marker)
	if idx < 0 {
		body := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "## Body"))
		return strings.TrimSpace(body), nil
	}
	bodyPart := strings.TrimSpace(raw[:idx])
	bodyPart = strings.TrimSpace(strings.TrimPrefix(bodyPart, "## Body"))
	var revs []Revision
	for _, line := range strings.Split(raw[idx:], "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(line, "- "), " | ", 3)
		if len(parts) < 3 {
			continue
		}
		at, _ := time.Parse(time.RFC3339, strings.TrimSpace(parts[0]))
		revs = append(revs, Revision{At: at, Actor: strings.TrimSpace(parts[1]), Summary: strings.TrimSpace(parts[2])})
	}
	return strings.TrimSpace(bodyPart), revs
}

func quoteAll(xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = fmt.Sprintf("%q", x)
	}
	return out
}

func parseList(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.Trim(strings.TrimSpace(p), `"`)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
