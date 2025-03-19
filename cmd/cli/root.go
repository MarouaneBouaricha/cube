/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cube",
	Short: "A Cli to interact with cube orchestrator",
	Long: `A Cli to interact with cube orchestrator.

Cube is an orchestrator based on containers.`,
}

func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		return err
	}
	return nil
}
