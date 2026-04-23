package main

import (
	"fmt"
	"os"

	"github.com/kefan/every-mysql-cli/internal/generator"
	"github.com/spf13/cobra"
)

var (
	host     string
	port     int
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
	initCmd.Flags().IntVar(&port, "port", 3306, "MySQL port")
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

	fmt.Printf("Scanning schema from %s@%s:%d/%s...\n", user, host, port, database)

	schema, err := generator.Scan(host, port, user, password, database)
	if err != nil {
		return fmt.Errorf("scan schema: %w", err)
	}

	fmt.Printf("Found %d tables: %v\n", len(schema.Tables), tableNames(schema))

	fmt.Printf("Generating CLI to %s...\n", output)
	if err := generator.Generate(schema, output); err != nil {
		return fmt.Errorf("generate CLI: %w", err)
	}

	fmt.Printf("CLI binary built at %s/%s-cli\n", output, database)
	fmt.Printf("Usage: %s/%s-cli <table> <action> [flags]\n", output, database)
	return nil
}

func tableNames(schema *generator.Schema) []string {
	names := make([]string, len(schema.Tables))
	for i, t := range schema.Tables {
		names[i] = t.Name
	}
	return names
}