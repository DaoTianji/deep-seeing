package story

import "errors"

// These classifications describe the actual bundled text editions, not the
// language of our Chinese UI titles. New catalog entries require an explicit
// provenance decision; English text alone does not imply a translation.
func catalogAuthorWork(b Book) (AuthorWork, error) {
	w := AuthorWork{Title: b.Title, Text: b.Text, Source: b.SourceURL, Genre: "短篇小说", Kind: "original", Language: "en", Attribution: "内置原作文本；具体历史版本见原文来源"}
	switch b.ID {
	case "necklace":
		w.Kind = "translation"
		w.Attribution = "内置历史英译本；风格须区分译者与原作者"
	case "kong":
		w.Language = "zh"
	case "magi", "leaf", "paw":
		// Verified English originals: see texts/README.md.
	default:
		return AuthorWork{}, errors.New("该内置作品的原作/译文归属尚未核验，不能自动导入作者档案")
	}
	return w, nil
}
