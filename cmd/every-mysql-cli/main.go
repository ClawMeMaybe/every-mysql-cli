package main

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "every-mysql-cli",
	Short: "Generate a standalone MySQL CLI tailored to your database schema",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}