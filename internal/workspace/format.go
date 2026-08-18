package workspace

import (
	"fmt"
	"strings"
	"time"
)

func formatDocument(d Document) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", d.ID)
	fmt.Fprintf(&b, "type: %s\n", d.Type)
	fmt.Fprintf(&b, "status: %s\n", d.Status)
	fmt.Fprintf(&b, "title: %q\n", d.Title)
	fmt.Fprintf(&b, "summary: %q\n", d.Summary)
	fmt.Fprintf(&b, "created_at: %s\n", d.CreatedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "updated_at: %s\n", d.UpdatedAt.UTC().Format(time.RFC3339))
	if len(d.EpisodeIDs) > 0 {
		fmt.Fprintf(&b, "episode_ids: [%s]\n", strings.Join(quoteAll(d.EpisodeIDs), ", "))
	}
	if len(d.RelatedSelfIDs) > 0 {
		fmt.Fprintf(&b, "related_self_ids: [%s]\n", strings.Join(quoteAll(d.RelatedSelfIDs), ", "))
	}
	b.WriteString("---\n\n")
	b.WriteString("## Body\n\n")
	b.WriteString(strings.TrimSpace(d.Body))
	b.WriteString("\n\n## Revisions\n\n")
	for _, r := range d.Revisions {
		fmt.Fprintf(&b, "- %s | %s | %s\n", r.At.UTC().Format(time.RFC3339), r.Actor, r.Summary)
	}
	return b.String()
}

func parseDocument(fallbackID, raw string) (Document, error) {
	d := Document{ID: fallbackID, Type: TypeQuestion, Status: StatusOpen}
	body := raw
	if strings.HasPrefix(raw, "---\n") {
		parts := strings.SplitN(raw, "---\n", 3)
		if len(parts) >= 3 {
			for _, line := range strings.Split(parts[1], "\n") {
				line = strings.TrimSpace(line)
				if after, ok := strings.CutPrefix(line, "id:"); ok {
					d.ID = strings.TrimSpace(after)
				}
				if after, ok := strings.CutPrefix(line, "type:"); ok {
					d.Type = NormalizeType(after)
				}
				if after, ok := strings.CutPrefix(line, "status:"); ok {
					d.Status = NormalizeStatus(after)
				}
				if after, ok := strings.CutPrefix(line, "title:"); ok {
					d.Title = strings.Trim(strings.TrimSpace(after), `"`)
				}
				if after, ok := strings.CutPrefix(line, "summary:"); ok {
					d.Summary = strings.Trim(strings.TrimSpace(after), `"`)
				}
				if after, ok := strings.CutPrefix(line, "episode_ids:"); ok {
					d.EpisodeIDs = parseList(after)
				}
				if after, ok := strings.CutPrefix(line, "related_self_ids:"); ok {
					d.RelatedSelfIDs = parseList(after)
				}
				if after, ok := strings.CutPrefix(line, "created_at:"); ok {
					if t, err := time.Parse(time.RFC3339, strings.TrimSpace(after)); err == nil {
						d.CreatedAt = t
					}
				}
				if after, ok := strings.CutPrefix(line, "updated_at:"); ok {
					if t, err := time.Parse(time.RFC3339, strings.TrimSpace(after)); err == nil {
						d.UpdatedAt = t
					}
				}
			}
			body = parts[2]
		}
	}
	d.Body, d.Revisions = splitBodyRevisions(body)
	if d.ID == "" {
		d.ID = fallbackID
	}
	return d, nil
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
