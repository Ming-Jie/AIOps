package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	termFileOnce sync.Once
	termFileDir  string
	genFileMu    sync.RWMutex
	genFileBase  string
)

const termFileTTL = 30 * time.Minute

func termFileStorageDir() (string, error) {
	var err error
	termFileOnce.Do(func() {
		termFileDir = filepath.Join(os.TempDir(), "gotest-termfiles")
		err = os.MkdirAll(termFileDir, 0755)
	})
	return termFileDir, err
}

// TermFileDir returns the terminal file storage directory, initializing it if needed.
func TermFileDir() string {
	d, _ := termFileStorageDir()
	return d
}

func init() {
	startTermFileCleanup()
}

// SaveTerminalFile saves a file to the temp storage and returns the download path.
// The path is relative to the API endpoint, e.g. "abc123/random_numbers.txt".
func SaveTerminalFile(filename string, data []byte) (string, error) {
	dir, err := termFileStorageDir()
	if err != nil {
		return "", err
	}

	uid := fmt.Sprintf("%d", time.Now().UnixNano())
	saveDir := filepath.Join(dir, uid)
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return "", err
	}

	dst := filepath.Join(saveDir, filename)
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return "", err
	}

	return uid + "/" + filename, nil
}

// SetGeneratedFileBase sets the persistent root for web chat generated files (under {base}/chat-generated/).
func SetGeneratedFileBase(dir string) {
	genFileMu.Lock()
	defer genFileMu.Unlock()
	genFileBase = strings.TrimSpace(dir)
}

func generatedFileStorageDir() (string, error) {
	genFileMu.RLock()
	base := genFileBase
	genFileMu.RUnlock()
	if base == "" {
		return "", fmt.Errorf("generated file base not configured")
	}
	root := filepath.Join(base, "chat-generated")
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", err
	}
	return root, nil
}

// SavePersistentGeneratedFile stores a web chat attachment under uploads/chat-generated/.
// Returns a path relative to /api/v1/chat/files/, e.g. chat-generated/uid/file.txt.
func SavePersistentGeneratedFile(filename string, data []byte) (string, error) {
	root, err := generatedFileStorageDir()
	if err != nil {
		return "", err
	}
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid filename")
	}
	uid := fmt.Sprintf("%d", time.Now().UnixNano())
	saveDir := filepath.Join(root, uid)
	if err := os.MkdirAll(saveDir, 0o750); err != nil {
		return "", err
	}
	dst := filepath.Join(saveDir, name)
	dst = filepath.Clean(dst)
	if !strings.HasPrefix(dst, filepath.Clean(saveDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid file path")
	}
	if err := os.WriteFile(dst, data, 0o640); err != nil {
		return "", err
	}
	return "chat-generated/" + uid + "/" + name, nil
}

// SaveWebGeneratedFile prefers persistent storage, falling back to terminal temp files.
func SaveWebGeneratedFile(filename string, data []byte) (urlPath string, persistent bool, err error) {
	if rel, err := SavePersistentGeneratedFile(filename, data); err == nil {
		return "/api/v1/chat/files/" + rel, true, nil
	}
	rel, err := SaveTerminalFile(filename, data)
	if err != nil {
		return "", false, err
	}
	return "/api/v1/chat/terminal-files/" + rel, false, nil
}

// ResolveTerminalFilePath converts a URL path (e.g. "abc123/random_numbers.txt") to an absolute filesystem path.
func ResolveTerminalFilePath(relPath string) (string, error) {
	dir, err := termFileStorageDir()
	if err != nil {
		return "", err
	}
	full := filepath.Join(dir, relPath)
	full = filepath.Clean(full)
	if !strings.HasPrefix(full, filepath.Clean(dir)+string(os.PathSeparator)) && full != filepath.Clean(dir) {
		return "", fmt.Errorf("invalid file path")
	}
	return full, nil
}

// FindRecentTerminalFilePath returns the newest terminal temp file matching basename.
func FindRecentTerminalFilePath(basename string) (string, error) {
	name := filepath.Base(strings.TrimSpace(basename))
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid filename")
	}
	dir, err := termFileStorageDir()
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	type candidate struct {
		path string
		mod  time.Time
	}
	var best *candidate
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		full := filepath.Join(dir, e.Name(), name)
		full = filepath.Clean(full)
		if !strings.HasPrefix(full, filepath.Clean(dir)+string(os.PathSeparator)) {
			continue
		}
		info, err := os.Stat(full)
		if err != nil || info.IsDir() || info.Size() <= 0 {
			continue
		}
		if best == nil || info.ModTime().After(best.mod) {
			best = &candidate{path: full, mod: info.ModTime()}
		}
	}
	if best == nil {
		return "", fmt.Errorf("terminal file not found: %s", name)
	}
	return best.path, nil
}

func startTermFileCleanup() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			cleanupTermFiles()
		}
	}()
}

func cleanupTermFiles() {
	dir, err := termFileStorageDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > termFileTTL {
			os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}
