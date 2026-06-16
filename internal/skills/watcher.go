package skills

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

func WatchDir(ctx context.Context, loader *Loader, registry *Registry, onNewSkill func(def *SkillDefinition)) error {
	dir := loader.skillsDir
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create skills dir for watching: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create fsnotify watcher: %w", err)
	}

	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return fmt.Errorf("watch skills dir: %w", err)
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				handleEvent(event, dir, loader, registry, onNewSkill)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Warn("skill watcher error", "err", err)
			}
		}
	}()

	return nil
}

func handleEvent(event fsnotify.Event, skillsDir string, loader *Loader, registry *Registry, onNewSkill func(def *SkillDefinition)) {
	if event.Op&fsnotify.Create != fsnotify.Create {
		return
	}

	info, err := os.Stat(event.Name)
	if err != nil || !info.IsDir() {
		return
	}

	skillPath := event.Name
	skillFile := filepath.Join(skillPath, "SKILL.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		return
	}

	dirName := filepath.Base(skillPath)
	skillKey := fmt.Sprintf("builtin_skill.%s", dirName)
	if _, exists := registry.Get(skillKey); exists {
		return
	}

	def, err := loader.LoadSkill(skillPath)
	if err != nil {
		slog.Warn("failed to load new skill from watcher", "dir", dirName, "err", err)
		return
	}
	registry.Register(def)
	slog.Info("hot-loaded new skill", "name", def.Name, "key", def.Key)
	if onNewSkill != nil {
		onNewSkill(def)
	}
}

func IsSkillsDirEntry(fi os.FileInfo) bool {
	if !fi.IsDir() {
		return false
	}
	name := strings.ToLower(fi.Name())
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return false
	}
	return true
}
