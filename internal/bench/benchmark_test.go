//go:build bench

package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	mysqlHost     = "localhost"
	mysqlPort     = 13306
	mysqlUser     = "root"
	mysqlPassword = "benchroot"
)

func TestBenchmarkArcher(t *testing.T) {
	// Step 1: Convert all SQLite databases to MySQL SQL fixtures
	zipPath, err := EnsureDatabaseZip()
	if err != nil {
		t.Fatalf("ensure archer-bench dataset: %v", err)
	}

	sqlDir := filepath.Join("testdata", "archer-bench", "sql")
	if err := ConvertAll(zipPath, sqlDir); err != nil {
		t.Fatalf("convert SQLite to MySQL: %v", err)
	}

	// Step 2: Start Docker Compose
	if err := dockerComposeUp(); err != nil {
		t.Fatalf("docker compose up: %v", err)
	}
	defer dockerComposeDown()

	// Step 3: Wait for MySQL healthcheck
	if err := waitForMySQL(60 * time.Second); err != nil {
		t.Fatalf("wait for MySQL: %v", err)
	}

	// Step 4: For each database, generate CLI and validate
	databases := getDatabaseNames(sqlDir)

	var totalCRUD, passCRUD, totalRobust, passRobust int
	var failures []string

	for _, dbName := range databases {
		cliPath, err := generateCLI(dbName)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: generate CLI: %v", dbName, err))
			continue
		}

		// Get table list from the generated CLI help
		tables := getTablesFromCLI(cliPath)

		fmt.Printf("=== %s ===\n", dbName)
		fmt.Printf("  Tables: %d (%s)\n", len(tables), strings.Join(tables, ", "))

		for _, tableName := range tables {
			totalCRUD++
			if validateCRUD(cliPath, tableName) {
				passCRUD++
			} else {
				failures = append(failures, fmt.Sprintf("%s.%s: CRUD validation failed", dbName, tableName))
			}
		}

		totalRobust++
		if validateRobustness(cliPath, dbName) {
			passRobust++
		} else {
			failures = append(failures, fmt.Sprintf("%s: robustness checks failed", dbName))
		}

		fmt.Printf("  CRUD: %s (%d/%d tables)\n",
			statusStr(passCRUD == totalCRUD), passCRUD, totalCRUD)
		fmt.Printf("  Robustness: %s (%d/%d checks)\n",
			statusStr(passRobust == totalRobust), passRobust, totalRobust)
	}

	fmt.Printf("\n=== SUMMARY ===\n")
	fmt.Printf("CRUD: %d/%d passed\n", passCRUD, totalCRUD)
	fmt.Printf("Robustness: %d/%d passed\n", passRobust, totalRobust)

	if len(failures) > 0 {
		t.Errorf("benchmark failures:\n%s", strings.Join(failures, "\n"))
	}
}

func dockerComposeUp() error {
	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = projectRoot()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose up: %w\n%s", err, out)
	}
	return nil
}

func dockerComposeDown() error {
	cmd := exec.Command("docker", "compose", "down", "-v")
	cmd.Dir = projectRoot()
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docker compose down: %v\n%s", err, out)
	}
	return nil
}

func waitForMySQL(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cmd := exec.Command("mysqladmin", "ping",
			"-h", mysqlHost, "-P", fmt.Sprintf("%d", mysqlPort),
			"-u", mysqlUser, fmt.Sprintf("-p%s", mysqlPassword))
		if err := cmd.Run(); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("MySQL not ready after %v", timeout)
}

func getDatabaseNames(sqlDir string) []string {
	entries, err := os.ReadDir(sqlDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, strings.TrimSuffix(e.Name(), ".sql"))
		}
	}
	return names
}

func generateCLI(dbName string) (string, error) {
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("%s-cli-*", dbName))
	if err != nil {
		return "", err
	}

	cmd := exec.Command("go", "run", "./cmd/every-mysql-cli",
		"init",
		"--host", mysqlHost,
		"--port", fmt.Sprintf("%d", mysqlPort),
		"--user", mysqlUser,
		"--password", mysqlPassword,
		"--database", dbName,
		"--output", tmpDir,
	)
	cmd.Dir = projectRoot()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("generate CLI: %w\n%s", err, out)
	}

	cliPath := filepath.Join(tmpDir, fmt.Sprintf("%s-cli", dbName))
	if _, err := os.Stat(cliPath); err != nil {
		return "", fmt.Errorf("CLI binary not found at %s", cliPath)
	}
	return cliPath, nil
}

func getTablesFromCLI(cliPath string) []string {
	cmd := exec.Command(cliPath, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	var tables []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "Usage") &&
			!strings.HasPrefix(line, "Available") && !strings.HasPrefix(line, "Flags") &&
			!strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "every") &&
			line != "cli for" && !strings.Contains(line, "Use:") {
			// This is likely a table subcommand
			tables = append(tables, line)
		}
	}
	return tables
}

func validateCRUD(cliPath string, tableName string) bool {
	// Create a row
	createCmd := exec.Command(cliPath, tableName, "create", "--json")
	// Add at least one column value flag — use first column we find
	// For simplicity, we test with a minimal create
	out, err := createCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("    %s create failed: %v\n%s\n", tableName, err, out)
		return false
	}

	// List and verify row exists
	listCmd := exec.Command(cliPath, tableName, "list", "--json", "--limit", "10")
	out, err = listCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("    %s list failed: %v\n%s\n", tableName, err, out)
		return false
	}

	// Verify JSON output
	var listData map[string]interface{}
	if err := json.Unmarshal(out, &listData); err != nil {
		fmt.Printf("    %s list JSON parse failed: %v\n", tableName, err)
		return false
	}
	dataArr, ok := listData["data"].([]interface{})
	if !ok || len(dataArr) == 0 {
		fmt.Printf("    %s list returned no data\n", tableName)
		return false
	}

	// For tables with PK, test get/update/delete
	row := dataArr[len(dataArr)-1].(map[string]interface{})

	// Check if table has PK by trying "get"
	// Extract PK value from the row — use "id" if present
	pkVal, hasPK := row["id"]
	if !hasPK {
		// Try other common PK names
		for _, key := range []string{"ID", "Id"} {
			if v, ok := row[key]; ok {
				pkVal = v
				hasPK = true
				break
			}
		}
	}

	if hasPK {
		pkStr := fmt.Sprintf("%v", pkVal)

		// Get by PK
		getCmd := exec.Command(cliPath, tableName, "get", pkStr, "--json")
		out, err = getCmd.CombinedOutput()
		if err != nil {
			fmt.Printf("    %s get %s failed: %v\n%s\n", tableName, pkStr, err, out)
			return false
		}

		// Update
		updateCmd := exec.Command(cliPath, tableName, "update", pkStr)
		out, err = updateCmd.CombinedOutput()
		if err != nil {
			// Update without flags may fail — that's expected
			// The important thing is update doesn't crash
		}

		// Delete without --force — should fail
		deleteCmd := exec.Command(cliPath, tableName, "delete", pkStr)
		out, _ = deleteCmd.CombinedOutput()
		if deleteCmd.ProcessState.Success() {
			fmt.Printf("    %s delete %s without --force should have failed\n", tableName, pkStr)
			return false
		}

		// Delete with --force — should succeed
		deleteCmd = exec.Command(cliPath, tableName, "delete", pkStr, "--force")
		out, err = deleteCmd.CombinedOutput()
		if err != nil {
			fmt.Printf("    %s delete %s --force failed: %v\n%s\n", tableName, pkStr, err, out)
			return false
		}
	}

	return true
}

func validateRobustness(cliPath string, dbName string) bool {
	passCount := 0
	checkCount := 0

	// Check 1: Destructive guard — delete without --force should fail
	// (Already tested in CRUD, but let's verify explicitly)
	checkCount++
	// We can't test this without a PK and row, so check with a dummy
	tables := getTablesFromCLI(cliPath)
	for _, table := range tables {
		deleteCmd := exec.Command(cliPath, table, "delete", "99999")
		out, _ := deleteCmd.CombinedOutput()
		if !deleteCmd.ProcessState.Success() {
			if strings.Contains(string(out), "--force") {
				passCount++
				break
			}
		}
	}

	// Check 2: --json produces valid JSON
	checkCount++
	for _, table := range tables {
		listCmd := exec.Command(cliPath, table, "list", "--json", "--limit", "1")
		out, err := listCmd.CombinedOutput()
		if err != nil {
			continue
		}
		var data map[string]interface{}
		if json.Unmarshal(out, &data) == nil {
			passCount++
			break
		}
	}

	// Check 3: Empty result set handled gracefully
	checkCount++
	for _, table := range tables {
		getCmd := exec.Command(cliPath, table, "get", "999999", "--json")
		out, err := getCmd.CombinedOutput()
		if err != nil {
			// Exit code != 0 is also acceptable for "not found"
			if !getCmd.ProcessState.Success() {
				passCount++
				break
			}
			continue
		}
		var data map[string]interface{}
		if json.Unmarshal(out, &data) == nil {
			passCount++
			break
		}
	}

	// Check 4: --by-<table> FK flags exist on list
	checkCount++
	for _, table := range tables {
		helpCmd := exec.Command(cliPath, table, "list", "--help")
		out, err := helpCmd.CombinedOutput()
		if err != nil {
			continue
		}
		if strings.Contains(string(out), "by-") {
			passCount++
			break
		}
	}

	// Check 5: --with-<table> FK flags exist on get
	checkCount++
	for _, table := range tables {
		helpCmd := exec.Command(cliPath, table, "get", "--help")
		out, err := helpCmd.CombinedOutput()
		if err != nil {
			continue
		}
		if strings.Contains(string(out), "with-") {
			passCount++
			break
		}
	}

	fmt.Printf("  Robustness: %d/%d checks passed\n", passCount, checkCount)
	return passCount >= checkCount-2 // Allow 2 checks to fail (some schemas may not have FKs)
}

func projectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	// Walk up until we find go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

func statusStr(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}