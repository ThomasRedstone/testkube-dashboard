package artifacts

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Manager struct {
	cacheDir string
	cacheTTL time.Duration
}

func NewManager(cacheDir string, cacheTTL time.Duration) *Manager {
	return &Manager{
		cacheDir: cacheDir,
		cacheTTL: cacheTTL,
	}
}

func (m *Manager) GetCachedReport(executionID string) (string, error) {
	path := filepath.Join(m.cacheDir, executionID)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // Not cached
		}
		return "", err
	}

	if time.Since(info.ModTime()) > m.cacheTTL {
		os.RemoveAll(path)
		return "", nil // Expired
	}

	return path, nil
}

func (m *Manager) SaveArtifacts(executionID string, data []byte) (string, error) {
	targetDir := filepath.Join(m.cacheDir, executionID)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Assume data is a zip file for now, since spec says "playwright-report/**/*"
	// In a real impl, we'd handle single files vs zips
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("failed to read zip: %w", err)
	}

	for _, f := range r.File {
		fpath := filepath.Join(targetDir, f.Name)

		// Zip Slip protection
		if !strings.HasPrefix(fpath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			return "", fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return "", err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return "", err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return "", err
		}

		// Security: Limit file size to prevent decompression bombs?
		// For now, Zip Slip is the main concern raised.
		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return "", err
		}
	}

	return targetDir, nil
}
