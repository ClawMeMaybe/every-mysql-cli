package main

import (
	"os"

	"github.com/kefan/every-mysql-cli/internal/generator"
	"github.com/spf13/cobra"
)

var cfg generator.Config

var rootCmd = &cobra.Command{
	Use:   "every-mysql-cli",
	Short: "Generate a standalone MySQL CLI tailored to your database schema",
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Connect to MySQL, scan schema, and generate a CLI binary",
	RunE: func(cmd *cobra.Command, args []string) error {
		return generator.Generate(cfg)
	},
}

func init() {
	initCmd.Flags().StringVar(&cfg.Host, "host", "localhost", "MySQL host")
	initCmd.Flags().IntVar(&cfg.Port, "port", 3306, "MySQL port")
	initCmd.Flags().StringVar(&cfg.User, "user", "root", "MySQL user")
	initCmd.Flags().StringVar(&cfg.Password, "password", "", "MySQL password")
	initCmd.Flags().StringVar(&cfg.Database, "database", "", "MySQL database name (required)")
	initCmd.Flags().StringVar(&cfg.Output, "output", "", "Output directory (default: <database>-cli)")

	initCmd.MarkFlagRequired("database")

	rootCmd.AddCommand(initCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}