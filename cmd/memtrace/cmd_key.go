package main

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Basekick-Labs/memtrace/internal/auth"
	"github.com/spf13/cobra"
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage API keys",
}

// --- create ---

var (
	keyCreateOrg         string
	keyCreateName        string
	keyCreatePermissions string
)

var keyCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key bound to an organization",
	RunE: func(cmd *cobra.Command, args []string) error {
		if keyCreateOrg == "" {
			return errors.New("--org is required")
		}
		if keyCreateName == "" {
			return errors.New("--name is required")
		}

		a, err := loadAdminContext(cmd.Context(), false)
		if err != nil {
			return err
		}
		defer a.close()

		exists, err := orgExists(a.db.GetDB(), keyCreateOrg)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("org '%s' not found", keyCreateOrg)
		}

		mgr := auth.NewManager(a.db.GetDB(), a.logger)
		defer mgr.Close()

		key, err := mgr.CreateKey(keyCreateName, keyCreateOrg, keyCreatePermissions)
		if err != nil {
			return err
		}

		fmt.Println("API key created (shown only once — save it now):")
		fmt.Println(key)
		return nil
	},
}

// --- list ---

var keyListOrg string

var keyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys for an organization",
	RunE: func(cmd *cobra.Command, args []string) error {
		if keyListOrg == "" {
			return errors.New("--org is required")
		}

		a, err := loadAdminContext(cmd.Context(), false)
		if err != nil {
			return err
		}
		defer a.close()

		mgr := auth.NewManager(a.db.GetDB(), a.logger)
		defer mgr.Close()

		keys, err := mgr.ListKeys(keyListOrg)
		if err != nil {
			return err
		}

		fmt.Printf("%-6s  %-30s  %-20s  %-10s  %s\n", "ID", "NAME", "PERMS", "ENABLED", "CREATED")
		for _, k := range keys {
			perms := ""
			for i, p := range k.Permissions {
				if i > 0 {
					perms += ","
				}
				perms += p
			}
			fmt.Printf("%-6d  %-30s  %-20s  %-10t  %s\n", k.ID, k.Name, perms, k.Enabled, k.CreatedAt.Format("2006-01-02 15:04"))
		}
		return nil
	},
}

// --- revoke ---

var keyRevokeCmd = &cobra.Command{
	Use:   "revoke <key_id>",
	Short: "Revoke (delete) an API key by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid key id: %w", err)
		}

		a, err := loadAdminContext(cmd.Context(), false)
		if err != nil {
			return err
		}
		defer a.close()

		mgr := auth.NewManager(a.db.GetDB(), a.logger)
		defer mgr.Close()

		if err := mgr.DeleteKey(id); err != nil {
			return err
		}
		fmt.Printf("Key %d revoked\n", id)
		return nil
	},
}

func init() {
	keyCreateCmd.Flags().StringVar(&keyCreateOrg, "org", "", "Organization ID (required)")
	keyCreateCmd.Flags().StringVar(&keyCreateName, "name", "", "Human-readable key name (required, unique)")
	keyCreateCmd.Flags().StringVar(&keyCreatePermissions, "permissions", "read,write", "Comma-separated permissions: read, write, admin")

	keyListCmd.Flags().StringVar(&keyListOrg, "org", "", "Organization ID (required)")

	keyCmd.AddCommand(keyCreateCmd)
	keyCmd.AddCommand(keyListCmd)
	keyCmd.AddCommand(keyRevokeCmd)
}
