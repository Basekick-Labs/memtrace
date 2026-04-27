package main

import (
	"fmt"

	memtracecrypto "github.com/Basekick-Labs/memtrace/internal/crypto"
	"github.com/spf13/cobra"
)

var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate cryptographic keys for Memtrace",
}

var keygenMasterCmd = &cobra.Command{
	Use:   "master",
	Short: "Print a fresh base64-encoded 32-byte master key for envelope encryption",
	Long: `Print a fresh base64-encoded 32-byte master key.

Set this value as the MEMTRACE_MASTER_KEY environment variable on every host
that runs the server or admin CLI. Memtrace uses it to encrypt per-org Arc API
keys at rest in the metadata database. Losing the key makes encrypted secrets
unrecoverable; rotate it deliberately, not casually.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := memtracecrypto.GenerateMasterKey()
		if err != nil {
			return err
		}
		fmt.Println(key)
		return nil
	},
}

func init() {
	keygenCmd.AddCommand(keygenMasterCmd)
}
