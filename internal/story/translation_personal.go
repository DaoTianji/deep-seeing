package story

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Personal revisions never mutate the shared edition. A whole-book revision
// remains a draft until every sentence has been translated successfully.
func (e *Engine) Retranslate(ctx context.Context, owner, style, sentence, revision, runID string, expected int) (TranslationEdition, error) {
	path, err := e.translationPath(owner, style)
	if err != nil {
		return TranslationEdition{}, err
	}
	key := "translation:personal:" + owner + ":" + style
	e.mu.Lock()
	if e.running[key] {
		e.mu.Unlock()
		return TranslationEdition{}, errors.New("个人译本正在生成，请稍后继续")
	}
	e.running[key] = true
	e.mu.Unlock()
	defer func() { e.mu.Lock(); delete(e.running, key); e.mu.Unlock() }()
	current, err := e.loadTranslation(owner, style)
	if err != nil {
		return current, err
	}
	if revision == "" || revision != current.Revision {
		return current, errors.New("译本已经变化，请刷新后重试")
	}
	if e.Chat == nil || e.Mode == "off" {
		return current, errors.New("翻译模型未启用，原有译本不受影响")
	}
	if sentence != "" {
		if current.Pending != nil {
			return current, errors.New("请先完成正在进行的整篇重译")
		}
		index := -1
		for i, s := range current.Sentences {
			if s.ID == sentence {
				index = i
				break
			}
		}
		if index < 0 {
			return current, ErrNotFound
		}
		if current.Completed != current.Total {
			return current, errors.New("请先完成默认译本，再重译单句")
		}
		if err = e.translateSelection(ctx, &current, index, index+1); err != nil {
			return current, err
		}
		current.Scope = ""
		current.Revision = ""
		current.Pending = nil
		if err = atomicJSON(path, current); err != nil {
			return current, err
		}
		return e.loadTranslation(owner, style)
	}
	var draft translationDraft
	raw, readErr := os.ReadFile(path + ".pending")
	if readErr == nil {
		if err = json.Unmarshal(raw, &draft); err != nil {
			return current, err
		}
	} else if !os.IsNotExist(readErr) {
		return current, readErr
	}
	if runID == "" {
		// Another initial request may arrive after the first committed its batch.
		// Return the same job; never restart and charge the first batch again.
		if draft.ID != "" {
			return current, nil
		}
		if expected != 0 {
			return current, errors.New("重译进度无效")
		}
		draft = translationDraft{ID: uuid.NewString(), Edition: current}
		draft.Edition.Pending = nil
		draft.Edition.Scope = ""
		draft.Edition.Revision = ""
		draft.Edition.Completed = 0
		for i := range draft.Edition.Sentences {
			draft.Edition.Sentences[i].Translation = ""
		}
		if err = atomicJSON(path+".pending", draft); err != nil {
			return current, err
		}
	} else if draft.ID != runID {
		return current, errors.New("重译任务已变化，请刷新后重试")
	}
	if expected < 0 || expected > draft.Edition.Completed {
		return current, errors.New("重译进度已变化")
	}
	if expected < draft.Edition.Completed {
		return e.loadTranslation(owner, style)
	}
	start := draft.Edition.Completed
	end := start
	chars := 0
	for end < draft.Edition.Total && end-start < 16 {
		n := len([]rune(draft.Edition.Sentences[end].Original))
		if n > 8000 {
			return current, errors.New("原文单句过长")
		}
		if end > start && chars+n > 6000 {
			break
		}
		chars += n
		end++
	}
	if start < end {
		if err = e.translateSelection(ctx, &draft.Edition, start, end); err != nil {
			return current, err
		}
		draft.Edition.Completed = end
		if err = atomicJSON(path+".pending", draft); err != nil {
			return current, err
		}
	}
	if draft.Edition.Completed == draft.Edition.Total {
		if err = atomicJSON(path, draft.Edition); err != nil {
			return current, err
		}
		// The completed edition is already durable; this removes only its job marker.
		if err = os.Remove(path + ".pending"); err != nil && !os.IsNotExist(err) {
			return current, err
		}
	}
	return e.loadTranslation(owner, style)
}

func (e *Engine) translateSelection(ctx context.Context, s *TranslationEdition, start, end int) error {
	inputs := append([]TranslationSentence(nil), s.Sentences[start:end]...)
	for i := range inputs {
		inputs[i].Translation = ""
	}
	var out struct {
		Items []struct {
			ID          string `json:"id"`
			Translation string `json:"translation"`
		} `json:"items"`
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := e.companionModel(ctx, translationPrompt, map[string]any{"title": e.Book.Title, "author": e.Book.Author, "style": s.Style, "glossary": translationGlossary(e.Book.ID), "sentences": inputs}, &out); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(out.Items) != len(inputs) {
		return errors.New("模型遗漏句子，本批未保存")
	}
	values := map[string]string{}
	for _, item := range out.Items {
		if _, exists := values[item.ID]; exists || strings.TrimSpace(item.Translation) == "" || len([]rune(item.Translation)) > 16000 {
			return errors.New("模型返回重复或无效译文")
		}
		values[item.ID] = strings.TrimSpace(item.Translation)
	}
	for _, input := range inputs {
		if _, ok := values[input.ID]; !ok {
			return errors.New("译文未与原句对齐")
		}
	}
	for i := start; i < end; i++ {
		s.Sentences[i].Translation = values[s.Sentences[i].ID]
	}
	s.Updated = time.Now().UTC()
	s.Model = e.Model
	return nil
}

// ExportDefaultTranslation is used by the explicit editorial seeding command.
// Only a complete edition with anchors validated against the book is exportable.
func (e *Engine) ExportDefaultTranslation(owner, style, path string) error {
	s, err := e.loadTranslationFile(owner, style, true)
	if err != nil {
		return err
	}
	if s.Completed != s.Total {
		return errors.New("默认译本尚未完成")
	}
	s.Scope = "default"
	s.Pending = nil
	s.Revision = ""
	return atomicJSON(path, s)
}
