package keytools

import (
	"os"
	"path/filepath"
)

const (
	defaultKeyFolder = ".ngkeys"
	defaultKeyFile   = "ngcore.key" // "~/.ngkeys/ngcore.key"
)

// GetDefault returns the default location storing keyfile
func GetDefault() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("failed to get home folder")
	}

	return filepath.Join(home, defaultKeyFolder, defaultKeyFile)
}
