package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Basekick-Labs/memtrace/internal/metadata"
	"github.com/spf13/cobra"
)

var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "Manage organizations and their Arc instance bindings",
}

// --- create ---

var orgCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new organization. Returns the generated org_id.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.TrimSpace(args[0])
		if name == "" {
			return errors.New("name must be non-empty")
		}

		a, err := loadAdminContext(cmd.Context(), false)
		if err != nil {
			return err
		}
		defer a.close()

		orgID := metadata.GenerateID("org_")
		if _, err := a.db.GetDB().ExecContext(cmd.Context(),
			`INSERT INTO organizations (id, name) VALUES (?, ?)`, orgID, name); err != nil {
			return fmt.Errorf("failed to create org: %w", err)
		}

		fmt.Printf("Organization created\n  id:   %s\n  name: %s\n", orgID, name)
		return nil
	},
}

// --- list ---

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all organizations",
	RunE: func(cmd *cobra.Command, args []string) error {
		a, err := loadAdminContext(cmd.Context(), false)
		if err != nil {
			return err
		}
		defer a.close()

		rows, err := a.db.GetDB().QueryContext(cmd.Context(),
			`SELECT id, name, created_at FROM organizations ORDER BY created_at ASC`)
		if err != nil {
			return err
		}
		defer rows.Close()

		fmt.Printf("%-32s  %-30s  %s\n", "ID", "NAME", "CREATED")
		for rows.Next() {
			var id, name, created string
			if err := rows.Scan(&id, &name, &created); err != nil {
				return err
			}
			fmt.Printf("%-32s  %-30s  %s\n", id, name, created)
		}
		return rows.Err()
	},
}

// --- delete ---

var orgDeleteCmd = &cobra.Command{
	Use:   "delete <org_id>",
	Short: "Delete an organization (cascades to its arc instance, agents, sessions, keys)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		orgID := args[0]

		a, err := loadAdminContext(cmd.Context(), false)
		if err != nil {
			return err
		}
		defer a.close()

		result, err := a.db.GetDB().ExecContext(cmd.Context(),
			`DELETE FROM organizations WHERE id = ?`, orgID)
		if err != nil {
			return err
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return fmt.Errorf("org '%s' not found", orgID)
		}
		fmt.Printf("Organization %s deleted\n", orgID)
		return nil
	},
}

// --- add-arc ---

var (
	addArcURL         string
	addArcAPIKey      string
	addArcDatabase    string
	addArcMeasurement string
)

var orgAddArcCmd = &cobra.Command{
	Use:   "add-arc <org_id>",
	Short: "Bind an Arc instance to an organization (encrypts the API key at rest)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		orgID := args[0]
		if addArcURL == "" {
			return errors.New("--url is required")
		}
		if addArcDatabase == "" {
			return errors.New("--database is required")
		}

		a, err := loadAdminContext(cmd.Context(), true)
		if err != nil {
			return err
		}
		defer a.close()

		exists, err := orgExists(a.db.GetDB(), orgID)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("org '%s' not found; create it first with `memtrace org create`", orgID)
		}

		if err := a.store.Create(cmd.Context(), &metadata.ArcInstance{
			OrgID:       orgID,
			URL:         addArcURL,
			APIKey:      addArcAPIKey,
			Database:    addArcDatabase,
			Measurement: addArcMeasurement,
		}); err != nil {
			return err
		}

		fmt.Printf("Arc instance bound to %s\n  url:      %s\n  database: %s\n", orgID, addArcURL, addArcDatabase)
		return nil
	},
}

// --- show-arc ---

var orgShowArcCmd = &cobra.Command{
	Use:   "show-arc <org_id>",
	Short: "Show the Arc instance for an org (API key masked)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		orgID := args[0]

		a, err := loadAdminContext(cmd.Context(), true)
		if err != nil {
			return err
		}
		defer a.close()

		inst, err := a.store.GetByOrg(cmd.Context(), orgID)
		if err != nil {
			if errors.Is(err, metadata.ErrArcInstanceNotFound) {
				return fmt.Errorf("no arc instance configured for %s", orgID)
			}
			return err
		}

		fmt.Printf("Arc instance for %s\n", orgID)
		fmt.Printf("  id:          %s\n", inst.ID)
		fmt.Printf("  url:         %s\n", inst.URL)
		fmt.Printf("  database:    %s\n", inst.Database)
		fmt.Printf("  measurement: %s\n", inst.Measurement)
		fmt.Printf("  api_key:     %s\n", maskAPIKey(inst.APIKey))
		return nil
	},
}

// --- remove-arc ---

var orgRemoveArcCmd = &cobra.Command{
	Use:   "remove-arc <org_id>",
	Short: "Remove the Arc instance binding for an org (the org itself is kept)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		orgID := args[0]

		a, err := loadAdminContext(cmd.Context(), true)
		if err != nil {
			return err
		}
		defer a.close()

		if err := a.store.Delete(cmd.Context(), orgID); err != nil {
			if errors.Is(err, metadata.ErrArcInstanceNotFound) {
				return fmt.Errorf("no arc instance configured for %s", orgID)
			}
			return err
		}
		fmt.Printf("Arc instance removed for %s\n", orgID)
		return nil
	},
}

func init() {
	orgAddArcCmd.Flags().StringVar(&addArcURL, "url", "", "Arc base URL (e.g. https://arc.example.com)")
	orgAddArcCmd.Flags().StringVar(&addArcAPIKey, "api-key", "", "Arc API key (encrypted at rest)")
	orgAddArcCmd.Flags().StringVar(&addArcDatabase, "database", "", "Arc database name")
	orgAddArcCmd.Flags().StringVar(&addArcMeasurement, "measurement", "events", "Arc measurement name")

	orgCmd.AddCommand(orgCreateCmd)
	orgCmd.AddCommand(orgListCmd)
	orgCmd.AddCommand(orgDeleteCmd)
	orgCmd.AddCommand(orgAddArcCmd)
	orgCmd.AddCommand(orgShowArcCmd)
	orgCmd.AddCommand(orgRemoveArcCmd)
}

// maskAPIKey shows only the prefix and length of an API key so operators can
// verify which key is bound without leaking it to logs or shoulder surfers.
func maskAPIKey(key string) string {
	if key == "" {
		return "(empty)"
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

