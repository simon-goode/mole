package mole

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type SessionState struct {
	TempDir string `json:"temp_dir"`
	DestDir string `json:"dest_dir"`
	Mode    string `json:"mode"`
}

func statePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".mole", "session.json"), nil
}

func saveState(state SessionState) error {
	path, err := statePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func loadState() (*SessionState, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

func clearState() error {
	path, err := statePath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func releaseFiles(destDirOverride string) error {
	state, err := loadState()
	if err != nil {
		return fmt.Errorf("no active session found. Run 'mole' first with 'safe' mode")
	}

	destDir := state.DestDir
	if destDirOverride != "" {
		destDir = destDirOverride
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("cannot create destination directory %s: %v", destDir, err)
	}

	entries, err := os.ReadDir(state.TempDir)
	if err != nil {
		clearState()
		return fmt.Errorf("cannot read temp directory (already cleaned up?): %v", err)
	}

	if len(entries) == 0 {
		fmt.Println("No queued files to release.")
		clearState()
		os.RemoveAll(state.TempDir)
		return nil
	}

	for _, entry := range entries {
		src := filepath.Join(state.TempDir, entry.Name())
		dst := filepath.Join(destDir, entry.Name())

		err := os.Rename(src, dst)
		if err != nil {
			data, err := os.ReadFile(src)
			if err != nil {
				return fmt.Errorf("error reading %s: %v", entry.Name(), err)
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				return fmt.Errorf("error writing %s: %v", entry.Name(), err)
			}
			os.Remove(src)
		}

		fmt.Printf("Released: %s\n", filepath.Base(dst))
	}

	os.RemoveAll(state.TempDir)
	clearState()

	fmt.Printf("All files released to: %s\n", destDir)

	return nil
}
