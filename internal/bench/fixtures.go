package bench

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const archerBenchURL = "https://sig4kg.github.io/archer-bench/dataset/database.zip"

func EnsureDatabaseZip() (string, error) {
	dataDir := filepath.Join("testdata", "archer-bench")
	zipPath := filepath.Join(dataDir, "database.zip")

	if _, err := os.Stat(zipPath); err == nil {
		return zipPath, nil
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}

	resp, err := http.Get(archerBenchURL)
	if err != nil {
		return "", fmt.Errorf("download archer-bench dataset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create zip file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		os.Remove(zipPath)
		return "", fmt.Errorf("write zip file: %w", err)
	}

	return zipPath, nil
}

func ExtractSQLiteFiles(zipPath string, destDir string) ([]string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("create dest dir: %w", err)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	var sqliteFiles []string
	for _, f := range r.File {
		if strings.Contains(f.Name, ".ipynb_checkpoints") {
			continue
		}
		// Only extract .sqlite files, skip .db files
		if !strings.HasSuffix(f.Name, ".sqlite") {
			continue
		}

		outPath := filepath.Join(destDir, filepath.Base(f.Name))
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s in zip: %w", f.Name, err)
		}

		out, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			return nil, fmt.Errorf("create %s: %w", outPath, err)
		}

		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("write %s: %w", outPath, err)
		}

		sqliteFiles = append(sqliteFiles, outPath)
	}

	return sqliteFiles, nil
}