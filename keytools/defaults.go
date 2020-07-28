package keytools

import (
	"os"
	"path/filepath"
)

const (
	defaultKeyFolder = ".ngkeys"
	defaultKeyFile   = "ngcore.key" // "~/.ngkeys/ngcore.key"
)

// GetDefaultFolder returns the default folder storing keyfile
func GetDefaultFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("failed to get home folder")
	}

	return filepath.Join(home, defaultKeyFolder)
}

// GetDefaultFile returns the default location storing keyfile
func GetDefaultFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic("failed to get home folder")
	}

	return filepath.Join(home, defaultKeyFolder, defaultKeyFile)
}
