package cmd

import (
	"github.com/labstack/echo/v4"
	"github.com/spf13/cobra"
	"github.com/woonglife62/woongkie-talkie/server/router"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start serve",
	Long: `
1. Start Simple Chat Server.`,
	Run: func(cmd *cobra.Command, args []string) {
		e := echo.New()

		router.Router(e)

		e.Logger.Fatal(e.Start(":8080"))
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
