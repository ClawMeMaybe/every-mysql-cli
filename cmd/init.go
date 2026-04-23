package main

import (
	"fmt"

	"github.com/kefan/every-mysql-cli/internal/generator"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	var host, port, user, password, database, output string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Connect to a MySQL database and generate a tailored CLI",
		Long:  "Scans a MySQL database schema and generates a standalone Go CLI binary with CRUD commands for each table.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if database == "" {
				return fmt.Errorf("--database is required")
			}
			if output == "" {
				output = "./" + database + "-cli"
			}

			// Step 1: Connect and scan
			fmt.Printf("Connecting to %s@%s:%s/%s ...\n", user, host, port, database)
			scanner, err := generator.NewScanner(host, port, user, password, database)
			if err != nil {
				return err
			}
			defer scanner.Close()

			fmt.Println("Scanning schema...")
			schema, err := scanner.Scan()
			if err != nil {
				return err
			}

			fmt.Printf("Found %d tables\n", len(schema.Tables))
			for _, t := range schema.Tables {
				fmt.Printf("  - %s (%d columns)\n", t.Name, len(t.Columns))
			}

			// Step 4-6: Generate project and build
			fmt.Printf("Generating CLI to %s ...\n", output)
			if err := generator.Generate(schema, output); err != nil {
				return err
			}

			// Step 7: Write config
			if err := generator.WriteConfig(schema, host, port, user, password); err != nil {
				fmt.Printf("Warning: could not write config file: %v\n", err)
			}

			// Step 8: Success message
			binaryPath := output + "/" + database + "-cli"
			fmt.Printf("\nSuccess! Generated CLI at %s\n", binaryPath)
			fmt.Printf("Usage example:\n  %s users list\n  %s users get 42\n  %s users list --json\n", binaryPath, binaryPath, binaryPath)

			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "localhost", "MySQL host")
	cmd.Flags().StringVar(&port, "port", "3306", "MySQL port")
	cmd.Flags().StringVar(&user, "user", "root", "MySQL user")
	cmd.Flags().StringVar(&password, "password", "", "MySQL password (consider DB_PASSWORD env var for security)")
	cmd.Flags().StringVar(&database, "database", "", "MySQL database name (required)")
	cmd.Flags().StringVar(&output, "output", "", "Output directory (default: ./<database>-cli)")

	return cmd
}