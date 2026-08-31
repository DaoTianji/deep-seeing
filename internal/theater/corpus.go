package theater

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

const (
	MaxRoleDocumentBytes = 32 << 20
	MaxRoleTextBytes     = 16 << 20
	MaxRoleChunkRunes    = 4000
)

type CorpusStore struct {
	root string
	db   *sql.DB
}

type CorpusIngestInput struct {
	RoleID       string
	CorpusRoleID string
	SourceID     string
	SourceURL    string
	Title        string
	MimeType     string
	Audience     SourceAudience
	Tier         SourceTier
	Text         []byte
}

func NewCorpusStore(root string) (*CorpusStore, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = filepath.Join("data", "memory", "roles")
	}
	for _, dir := range []string{"corpus/documents", "corpus/chunks"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			return nil, err
		}
	}
	dbPath := filepath.Join(root, "corpus-index.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS role_chunks_fts USING fts5(chunk_id UNINDEXED, corpus_role_id UNINDEXED, audience UNINDEXED, title, section, body, tokenize=unicode61)`); err != nil {
		db.Close()
		return nil, err
	}
	if err := os.Chmod(dbPath, 0o600); err != nil {
		db.Close()
		return nil, err
	}
	return &CorpusStore{root: root, db: db}, nil
}

func (c *CorpusStore) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *CorpusStore) Ingest(ctx context.Context, in CorpusIngestInput) (RoleDocument, []RoleChunk, bool, error) {
	if c == nil || c.db == nil {
		return RoleDocument{}, nil, false, fmt.Errorf("corpus store unavailable")
	}
	in.RoleID, in.CorpusRoleID, in.SourceID = cleanText(in.RoleID), cleanText(in.CorpusRoleID), cleanText(in.SourceID)
	in.SourceURL = canonicalRoleSourceURL(in.SourceURL)
	in.Title = cleanText(in.Title)
	if in.RoleID == "" || in.Title == "" {
		return RoleDocument{}, nil, false, fmt.Errorf("role_id and title required")
	}
	if in.CorpusRoleID == "" {
		in.CorpusRoleID = in.RoleID
	}
	if len(in.Text) > MaxRoleTextBytes {
		return RoleDocument{}, nil, false, fmt.Errorf("extracted role text exceeds 16 MiB")
	}
	if !utf8.Valid(in.Text) {
		return RoleDocument{}, nil, false, fmt.Errorf("role text must be valid UTF-8")
	}
	if in.Audience == "" {
		in.Audience = SourceActor
	}
	if in.Audience != SourceActor && in.Audience != SourceDirector {
		return RoleDocument{}, nil, false, fmt.Errorf("invalid source audience")
	}
	in.Tier = normalizeSourceTier(in.Tier)
	hash := sha256Hex(in.Text)
	if in.SourceURL != "" {
		if existing, found, err := c.findDocumentByURL(in.CorpusRoleID, in.SourceURL); err != nil {
			return RoleDocument{}, nil, false, err
		} else if found {
			chunks, readErr := c.ListDocumentChunks(ctx, existing.ID)
			return existing, chunks, true, readErr
		}
	}
	if existing, found, err := c.findDocumentByHash(in.CorpusRoleID, hash); err != nil {
		return RoleDocument{}, nil, false, err
	} else if found {
		chunks, readErr := c.ListDocumentChunks(ctx, existing.ID)
		return existing, chunks, true, readErr
	}
	now := time.Now().UTC()
	document := RoleDocument{
		ID: "rdoc_" + compactUUID(), RoleID: in.RoleID, CorpusRoleID: in.CorpusRoleID,
		SourceID: in.SourceID, SourceURL: in.SourceURL, Title: in.Title, MimeType: cleanText(in.MimeType),
		Audience: in.Audience, Tier: in.Tier, ContentHash: hash, CreatedAt: now,
	}
	documentDir := filepath.Join(c.root, "corpus", "documents", safeID(in.CorpusRoleID))
	if err := os.MkdirAll(documentDir, 0o700); err != nil {
		return RoleDocument{}, nil, false, err
	}
	document.Path = filepath.ToSlash(filepath.Join("corpus", "documents", safeID(in.CorpusRoleID), safeID(document.ID)+".txt"))
	if err := os.WriteFile(filepath.Join(c.root, filepath.FromSlash(document.Path)), in.Text, 0o600); err != nil {
		return RoleDocument{}, nil, false, err
	}
	pieces := splitCorpusText(string(in.Text))
	chunks := make([]RoleChunk, 0, len(pieces))
	chunkDir := filepath.Join(c.root, "corpus", "chunks", safeID(in.CorpusRoleID))
	if err := os.MkdirAll(chunkDir, 0o700); err != nil {
		return RoleDocument{}, nil, false, err
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return RoleDocument{}, nil, false, err
	}
	defer tx.Rollback()
	for _, piece := range pieces {
		chunk := RoleChunk{
			ID: "rchunk_" + compactUUID(), RoleID: in.RoleID, CorpusRoleID: in.CorpusRoleID,
			DocumentID: document.ID, SourceID: in.SourceID, Title: in.Title,
			Section: piece.section, Page: piece.page, Audience: in.Audience, Tier: in.Tier,
			Content: piece.content, ContentHash: sha256Hex([]byte(piece.content)), CreatedAt: now,
		}
		if err := writeJSONAtomic(filepath.Join(chunkDir, safeID(chunk.ID)+".json"), chunk); err != nil {
			return RoleDocument{}, nil, false, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO role_chunks_fts(chunk_id, corpus_role_id, audience, title, section, body) VALUES(?,?,?,?,?,?)`,
			chunk.ID, chunk.CorpusRoleID, string(chunk.Audience), chunk.Title, chunk.Section, chunk.Content); err != nil {
			return RoleDocument{}, nil, false, err
		}
		chunks = append(chunks, chunk)
	}
	document.ChunkCount = len(chunks)
	if err := writeJSONAtomic(c.documentMetaPath(document), document); err != nil {
		return RoleDocument{}, nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return RoleDocument{}, nil, false, err
	}
	return document, chunks, false, nil
}

type corpusPiece struct {
	section string
	page    int
	content string
}

func splitCorpusText(text string) []corpusPiece {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	pages := strings.Split(text, "\f")
	var out []corpusPiece
	var previous string
	section := ""
	for pageIndex, page := range pages {
		paragraphs := splitParagraphs(page)
		var current strings.Builder
		for _, paragraph := range paragraphs {
			if strings.HasPrefix(paragraph, "#") {
				if value := strings.TrimSpace(current.String()); value != "" {
					out = append(out, corpusPiece{section: section, page: pageIndex + 1, content: value})
					current.Reset()
				}
				previous = ""
				section = cleanText(strings.TrimLeft(paragraph, "#"))
				continue
			}
			runes := []rune(paragraph)
			for len(runes) > MaxRoleChunkRunes {
				part := string(runes[:MaxRoleChunkRunes])
				out = append(out, corpusPiece{section: section, page: pageIndex + 1, content: part})
				runes = runes[MaxRoleChunkRunes:]
				previous = part
			}
			paragraph = string(runes)
			nextSize := len([]rune(current.String())) + len([]rune(paragraph)) + 2
			if current.Len() > 0 && nextSize > MaxRoleChunkRunes {
				value := strings.TrimSpace(current.String())
				out = append(out, corpusPiece{section: section, page: pageIndex + 1, content: value})
				previous = paragraphTail(value, 600)
				current.Reset()
				if previous != "" {
					current.WriteString(previous)
					current.WriteString("\n\n")
				}
			}
			current.WriteString(paragraph)
			current.WriteString("\n\n")
		}
		if value := strings.TrimSpace(current.String()); value != "" {
			out = append(out, corpusPiece{section: section, page: pageIndex + 1, content: value})
			previous = paragraphTail(value, 600)
		}
	}
	if len(out) == 0 && strings.TrimSpace(text) != "" {
		out = append(out, corpusPiece{page: 1, content: strings.TrimSpace(text)})
	}
	return out
}

func splitParagraphs(text string) []string {
	var out []string
	for _, part := range strings.Split(text, "\n\n") {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func paragraphTail(text string, n int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= n {
		return string(runes)
	}
	return string(runes[len(runes)-n:])
}

func normalizeSourceTier(tier SourceTier) SourceTier {
	switch tier {
	case SourcePrimary, SourceContemporary, SourceBiography, SourceScholarship, SourcePosthumous, SourceGenerated:
		return tier
	default:
		return SourceScholarship
	}
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func canonicalRoleSourceURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.Fragment = ""
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Path != "/" {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	}
	return parsed.String()
}

func (c *CorpusStore) findDocumentByURL(corpusRoleID, sourceURL string) (RoleDocument, bool, error) {
	documents, err := c.ListDocuments(context.Background(), corpusRoleID)
	if err != nil {
		return RoleDocument{}, false, err
	}
	for _, document := range documents {
		if canonicalRoleSourceURL(document.SourceURL) == sourceURL {
			return document, true, nil
		}
	}
	return RoleDocument{}, false, nil
}

func (c *CorpusStore) findDocumentByHash(corpusRoleID, hash string) (RoleDocument, bool, error) {
	documents, err := c.ListDocuments(context.Background(), corpusRoleID)
	if err != nil {
		return RoleDocument{}, false, err
	}
	for _, document := range documents {
		if document.ContentHash == hash {
			return document, true, nil
		}
	}
	return RoleDocument{}, false, nil
}

func (c *CorpusStore) ListDocuments(_ context.Context, corpusRoleID string) ([]RoleDocument, error) {
	paths, err := filepath.Glob(filepath.Join(c.root, "corpus", "documents", safeID(corpusRoleID), "*.json"))
	if err != nil {
		return nil, err
	}
	var out []RoleDocument
	for _, path := range paths {
		var document RoleDocument
		if readJSON(path, &document) == nil {
			out = append(out, document)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (c *CorpusStore) ListDocumentChunks(_ context.Context, documentID string) ([]RoleChunk, error) {
	paths, err := filepath.Glob(filepath.Join(c.root, "corpus", "chunks", "*", "*.json"))
	if err != nil {
		return nil, err
	}
	var out []RoleChunk
	for _, path := range paths {
		var chunk RoleChunk
		if readJSON(path, &chunk) == nil && chunk.DocumentID == documentID {
			out = append(out, chunk)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Page != out[j].Page {
			return out[i].Page < out[j].Page
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (c *CorpusStore) ReadChunk(_ context.Context, id string) (RoleChunk, error) {
	paths, err := filepath.Glob(filepath.Join(c.root, "corpus", "chunks", "*", safeID(id)+".json"))
	if err != nil {
		return RoleChunk{}, err
	}
	if len(paths) == 0 {
		return RoleChunk{}, os.ErrNotExist
	}
	var chunk RoleChunk
	err = readJSON(paths[0], &chunk)
	return chunk, err
}

func (c *CorpusStore) Search(ctx context.Context, corpusRoleID, query string, audience SourceAudience, limit int) ([]RoleChunkCard, error) {
	if c == nil || c.db == nil {
		return nil, fmt.Errorf("corpus store unavailable")
	}
	rawQuery := cleanText(query)
	if rawQuery == "" {
		return []RoleChunkCard{}, nil
	}
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}
	var sqlText string
	var args []any
	literal := containsNonASCII(rawQuery)
	if literal {
		sqlText = `SELECT chunk_id, substr(body, 1, 240) FROM role_chunks_fts WHERE corpus_role_id = ? AND (`
		args = append(args, cleanText(corpusRoleID))
		var clauses []string
		for _, term := range strings.Fields(rawQuery) {
			clauses = append(clauses, "(body LIKE ? OR title LIKE ? OR section LIKE ?)")
			pattern := "%" + term + "%"
			args = append(args, pattern, pattern, pattern)
		}
		sqlText += strings.Join(clauses, " OR ") + ")"
	} else {
		sqlText = `SELECT chunk_id, snippet(role_chunks_fts, 5, '[', ']', '…', 24) FROM role_chunks_fts WHERE role_chunks_fts MATCH ? AND corpus_role_id = ?`
		args = append(args, ftsQuery(rawQuery), cleanText(corpusRoleID))
	}
	if audience != "" {
		sqlText += " AND audience = ?"
		args = append(args, string(audience))
	}
	if literal {
		sqlText += " ORDER BY rowid LIMIT ?"
	} else {
		sqlText += " ORDER BY rank LIMIT ?"
	}
	args = append(args, limit)
	rows, err := c.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RoleChunkCard
	for rows.Next() {
		var id, excerpt string
		if err := rows.Scan(&id, &excerpt); err != nil {
			return nil, err
		}
		chunk, err := c.ReadChunk(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, RoleChunkCard{
			ID: chunk.ID, DocumentID: chunk.DocumentID, SourceID: chunk.SourceID,
			Title: chunk.Title, Section: chunk.Section, Page: chunk.Page,
			Audience: chunk.Audience, Tier: chunk.Tier, Excerpt: excerpt,
		})
	}

	return out, rows.Err()
}

func ftsQuery(query string) string {
	var terms []string
	for _, term := range strings.Fields(cleanText(query)) {
		term = strings.ReplaceAll(term, `"`, "")
		if term != "" {
			terms = append(terms, `"`+term+`"`)
		}
	}
	return strings.Join(terms, " OR ")
}

func containsNonASCII(value string) bool {
	for _, r := range value {
		if r > 127 {
			return true
		}
	}
	return false
}

func (c *CorpusStore) Rebuild(ctx context.Context) error {
	if _, err := c.db.ExecContext(ctx, `DELETE FROM role_chunks_fts`); err != nil {
		return err
	}
	paths, err := filepath.Glob(filepath.Join(c.root, "corpus", "chunks", "*", "*.json"))
	if err != nil {
		return err

	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, path := range paths {
		var chunk RoleChunk
		if readJSON(path, &chunk) != nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO role_chunks_fts(chunk_id, corpus_role_id, audience, title, section, body) VALUES(?,?,?,?,?,?)`,
			chunk.ID, chunk.CorpusRoleID, string(chunk.Audience), chunk.Title, chunk.Section, chunk.Content); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (c *CorpusStore) documentMetaPath(document RoleDocument) string {
	return filepath.Join(c.root, "corpus", "documents", safeID(document.CorpusRoleID), safeID(document.ID)+".json")
}
