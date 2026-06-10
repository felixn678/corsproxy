// Package cmd defines the corsproxy CLI: flag parsing, the interactive
// fallback form, and server startup.
package cmd

import (
	"fmt"
	"strconv"

	"github.com/felix-nguyen/corsproxy/internal/config"
	"github.com/spf13/cobra"
)

var (
	flagTarget string
	flagPort   int
	flagOrigin string
)

var rootCmd = &cobra.Command{
	Use:   "corsproxy",
	Short: "CORS reverse proxy for local development",
	Long: `corsproxy runs a proxy server that forwards all requests to your backend
and injects permissive CORS headers, so other devices on the same LAN (e.g. a phone)
can call your local API without CORS errors.

Run without flags to enter interactive mode.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// No --target → interactive mode. huh works with strings, so the
		// port round-trips through a string and is re-validated below.
		if flagTarget == "" {
			portStr := strconv.Itoa(flagPort)
			if err := runInteractiveForm(&flagTarget, &portStr, &flagOrigin); err != nil {
				return fmt.Errorf("%w\n(no TTY? use flags instead: corsproxy --target http://localhost:8080)", err)
			}
			flagPort, _ = strconv.Atoi(portStr) // form already validated the number
		}

		cfg, err := config.New(flagTarget, flagPort, flagOrigin)
		if err != nil {
			return err
		}
		return runServer(cfg)
	},
}

func init() {
	rootCmd.Flags().StringVar(&flagTarget, "target", "", "backend URL to forward to, e.g. http://localhost:8080")
	rootCmd.Flags().IntVar(&flagPort, "port", 3001, "port the proxy listens on")
	rootCmd.Flags().StringVar(&flagOrigin, "origin", "*", `allowed origin ("*" = all, or a specific IP/origin)`)
}

// Execute runs the root command; main() exits non-zero on error.
func Execute() error {
	return rootCmd.Execute()
}
