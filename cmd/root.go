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
	Short: "CORS reverse proxy cho local development",
	Long: `corsproxy chạy một server trung gian: forward mọi request đến backend
và inject CORS headers, để thiết bị khác trong cùng mạng LAN (vd điện thoại)
gọi API local mà không dính lỗi CORS.

Chạy không có flags để vào chế độ interactive.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// No --target → interactive mode. huh works with strings, so the
		// port round-trips through a string and is re-validated below.
		if flagTarget == "" {
			portStr := strconv.Itoa(flagPort)
			if err := runInteractiveForm(&flagTarget, &portStr, &flagOrigin); err != nil {
				return fmt.Errorf("%w\n(không có terminal? dùng flags: corsproxy --target http://localhost:8080)", err)
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
	rootCmd.Flags().StringVar(&flagTarget, "target", "", "backend URL để forward đến, vd http://localhost:8080")
	rootCmd.Flags().IntVar(&flagPort, "port", 3001, "port proxy lắng nghe")
	rootCmd.Flags().StringVar(&flagOrigin, "origin", "*", `origin được phép ("*" = tất cả, hoặc IP/origin cụ thể)`)
}

// Execute runs the root command; main() exits non-zero on error.
func Execute() error {
	return rootCmd.Execute()
}
