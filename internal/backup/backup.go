package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Paths are relative roots under the memory tree to include in a snapshot.
var DefaultRoots = []string{
	"data/memory/episodes",
	"data/memory/proposals",
	"data/memory/mutations",
	"data/memory/state",
	"data/memory/traces",
	"seed/SOUL.md",
	"seed/origin",
}

// Snapshot copies memory-related trees into destDir (or data/backups/<timestamp>).
func Snapshot(workspace, destDir string, roots []string) (string, error) {
	if strings.TrimSpace(workspace) == "" {
		workspace = "."
	}
	if len(roots) == 0 {
		roots = DefaultRoots
	}
	if strings.TrimSpace(destDir) == "" {
		destDir = filepath.Join(workspace, "data", "backups", time.Now().UTC().Format("20060102T150405Z"))
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	var copied int
	for _, root := range roots {
		src := filepath.Join(workspace, root)
		info, err := os.Stat(src)
		if err != nil {
			continue // skip missing optional dirs
		}
		dst := filepath.Join(destDir, root)
		if info.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return "", fmt.Errorf("copy %s: %w", root, err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return "", err
			}
			if err := copyFile(src, dst); err != nil {
				return "", err
			}
		}
		copied++
	}
	meta := fmt.Sprintf("created_at: %s\nroots_copied: %d\n", time.Now().UTC().Format(time.RFC3339), copied)
	if err := os.WriteFile(filepath.Join(destDir, "BACKUP_META.txt"), []byte(meta), 0o644); err != nil {
		return "", err
	}
	return destDir, nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
