package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
)

var rootCmd = &cobra.Command{
	Use:   "",
	Short: "",
	Long:  ``,
	// Run: func(cmd *cobra.Command, args []string) {
	// 	log.Println("rootCmd test")
	// },
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Logger.Fatal(err.Error())
		os.Exit(1)
	}
}
