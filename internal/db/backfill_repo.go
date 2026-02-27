package db

import (
	"context"
	"database/sql"
	"time"
)

type BackfillRepo struct {
	db *sql.DB
}

func (r *BackfillRepo) GetState(ctx context.Context, guildID, channelID string) (string, bool, error) {
	row := r.db.QueryRowContext(ctx, `SELECT last_scanned_message_id FROM backfill_state WHERE guild_id = ? AND channel_id = ?`, guildID, channelID)
	var lastID sql.NullString
	if err := row.Scan(&lastID); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	if !lastID.Valid {
		return "", true, nil
	}
	return lastID.String, true, nil
}

func (r *BackfillRepo) UpsertState(ctx context.Context, guildID, channelID, lastMessageID string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO backfill_state(guild_id, channel_id, last_scanned_message_id, updated_at)
	VALUES(?, ?, ?, ?)
	ON CONFLICT(guild_id, channel_id) DO UPDATE SET last_scanned_message_id=excluded.last_scanned_message_id, updated_at=excluded.updated_at`,
		guildID, channelID, lastMessageID, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}
