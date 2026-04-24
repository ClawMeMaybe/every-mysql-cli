package main

import (
	"fmt"
	"os"

	"github.com/kefan/every-mysql-cli/internal/generator"
	"github.com/kefan/every-mysql-cli/internal/types"
	"github.com/spf13/cobra"
)

var (
	host     string
	port     string
	user     string
	password string
	database string
	output   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "every-mysql-cli",
		Short: "Generate a MySQL CRUD CLI from a database schema",
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Connect to MySQL, scan schema, generate a CLI binary",
		RunE:  runInit,
	}

	initCmd.Flags().StringVar(&host, "host", "localhost", "MySQL host")
	initCmd.Flags().StringVar(&port, "port", "3306", "MySQL port")
	initCmd.Flags().StringVar(&user, "user", "root", "MySQL user")
	initCmd.Flags().StringVar(&password, "password", "", "MySQL password")
	initCmd.Flags().StringVar(&database, "database", "", "MySQL database name (required)")
	initCmd.Flags().StringVar(&output, "output", "", "Output directory (default: ./<database>-cli)")

	rootCmd.AddCommand(initCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runInit(cmd *cobra.Command, args []string) error {
	if database == "" {
		return fmt.Errorf("--database is required")
	}
	if output == "" {
		output = fmt.Sprintf("./%s-cli", database)
	}

	fmt.Printf("Connecting to %s@%s:%s/%s ...\n", user, host, port, database)

	scanner, err := generator.NewScanner(host, port, user, password, database)
	if err != nil {
		return fmt.Errorf("scan schema: %w", err)
	}
	defer scanner.Close()

	fmt.Println("Scanning schema...")
	schema, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("scan schema: %w", err)
	}

	fmt.Printf("Found %d tables: %v\n", len(schema.Tables), tableNames(schema))

	fmt.Printf("Generating CLI to %s...\n", output)
	if err := generator.Generate(schema, output); err != nil {
		return fmt.Errorf("generate CLI: %w", err)
	}

	if err := generator.WriteConfig(schema, host, port, user, password); err != nil {
		fmt.Printf("Warning: could not write config file: %v\n", err)
	}

	binaryPath := output + "/" + database + "-cli"
	fmt.Printf("\nSuccess! Generated CLI at %s\n", binaryPath)
	fmt.Printf("Usage: %s <table> <action> [flags]\n", binaryPath)
	return nil
}

func tableNames(schema *types.Schema) []string {
	names := make([]string, len(schema.Tables))
	for i, t := range schema.Tables {
		names[i] = t.Name
	}
	return names
}