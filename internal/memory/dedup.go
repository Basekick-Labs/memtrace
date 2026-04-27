package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Basekick-Labs/memtrace/internal/arc"
	"github.com/Basekick-Labs/memtrace/internal/sanitize"
	"github.com/rs/zerolog"
)

// ErrDuplicate is returned when a memory is a duplicate
var ErrDuplicate = errors.New("duplicate memory")

// DedupEngine checks for duplicate memories before writing
type DedupEngine struct {
	arcRegistry *arc.Registry
	windowHours int
	logger      zerolog.Logger
}

// NewDedupEngine creates a new dedup engine
func NewDedupEngine(arcRegistry *arc.Registry, windowHours int, logger zerolog.Logger) *DedupEngine {
	return &DedupEngine{
		arcRegistry: arcRegistry,
		windowHours: windowHours,
		logger:      logger.With().Str("component", "dedup").Logger(),
	}
}

// GenerateKey creates a dedup key from agent, event type, and content
func GenerateKey(agentID, eventType, content string) string {
	// Use first 200 chars of content to catch identical actions
	truncated := content
	if len(truncated) > 200 {
		truncated = truncated[:200]
	}
	raw := agentID + "|" + eventType + "|" + truncated
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])[:32]
}

// Check returns ErrDuplicate if a memory with the same dedup key exists in the time window
func (d *DedupEngine) Check(ctx context.Context, orgID, dedupKey string) error {
	if dedupKey == "" {
		return nil
	}

	// orgID and dedupKey are validated upstream (orgID from auth, dedupKey is a hex hash),
	// but escape them for defense-in-depth.
	sql := fmt.Sprintf(
		"SELECT COUNT(*) as cnt FROM events WHERE org_id = '%s' AND dedup_key = '%s' AND time > (CURRENT_TIMESTAMP - INTERVAL '%d hours')",
		sanitize.EscapeSQL(orgID),
		sanitize.EscapeSQL(dedupKey),
		d.windowHours,
	)

	arcClient, err := d.arcRegistry.Get(orgID)
	if err != nil {
		// No Arc instance for this org — let the upstream write fail with the same error
		d.logger.Warn().Err(err).Msg("Dedup check skipped: no arc instance")
		return nil
	}
	rows, err := arcClient.Query(ctx, sql)
	if err != nil {
		// If Arc is unreachable, allow the write (fail open)
		d.logger.Warn().Err(err).Msg("Dedup check failed, allowing write")
		return nil
	}

	if len(rows) > 0 {
		if cnt, ok := rows[0]["cnt"]; ok {
			var count float64
			switch v := cnt.(type) {
			case float64:
				count = v
			case int64:
				count = float64(v)
			case json.Number:
				count, _ = v.Float64()
			}
			if count > 0 {
				d.logger.Debug().Str("dedup_key", dedupKey).Msg("Duplicate detected")
				return ErrDuplicate
			}
		}
	}

	return nil
}
