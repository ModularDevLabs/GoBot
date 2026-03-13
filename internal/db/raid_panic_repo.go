package db

import (
	"context"
	"database/sql"
	"time"
)

type RaidPanicLockdownRow struct {
	ID              int64      `json:"id"`
	GuildID         string     `json:"guild_id"`
	Status          string     `json:"status"`
	SlowmodeSeconds int        `json:"slowmode_seconds"`
	StartedBy       string     `json:"started_by"`
	StartedAt       time.Time  `json:"started_at"`
	EndsAt          time.Time  `json:"ends_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	EndReason       string     `json:"end_reason"`
}

type RaidPanicChannelStateRow struct {
	LockdownID              int64  `json:"lockdown_id"`
	GuildID                 string `json:"guild_id"`
	ChannelID               string `json:"channel_id"`
	PreviousSlowmodeSeconds int    `json:"previous_slowmode_seconds"`
}

type RaidPanicRepo struct {
	db *sql.DB
}

func (r *RaidPanicRepo) CreateLockdown(ctx context.Context, guildID string, slowmodeSeconds int, startedBy string, endsAt time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO raid_panic_lockdowns(guild_id, status, slowmode_seconds, started_by, started_at, ends_at)
		VALUES(?, 'active', ?, ?, ?, ?)`,
		guildID, slowmodeSeconds, startedBy, time.Now().UTC().Format(time.RFC3339), endsAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *RaidPanicRepo) AddChannelState(ctx context.Context, row RaidPanicChannelStateRow) error {
	_, err := r.db.ExecContext(ctx, `INSERT OR REPLACE INTO raid_panic_channel_states(lockdown_id, guild_id, channel_id, previous_slowmode_seconds)
		VALUES(?, ?, ?, ?)`,
		row.LockdownID, row.GuildID, row.ChannelID, row.PreviousSlowmodeSeconds,
	)
	return err
}

func (r *RaidPanicRepo) ActiveLockdownByGuild(ctx context.Context, guildID string) (RaidPanicLockdownRow, bool, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, guild_id, status, slowmode_seconds, started_by, started_at, ends_at, ended_at, end_reason
		FROM raid_panic_lockdowns WHERE guild_id = ? AND status = 'active' ORDER BY started_at DESC LIMIT 1`, guildID)
	return scanRaidPanicLockdown(row)
}

func (r *RaidPanicRepo) ListDueActiveLockdowns(ctx context.Context, now time.Time, limit int) ([]RaidPanicLockdownRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, guild_id, status, slowmode_seconds, started_by, started_at, ends_at, ended_at, end_reason
		FROM raid_panic_lockdowns
		WHERE status = 'active' AND ends_at <= ?
		ORDER BY ends_at ASC LIMIT ?`, now.UTC().Format(time.RFC3339), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RaidPanicLockdownRow, 0)
	for rows.Next() {
		item, err := scanRaidPanicLockdownFromRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *RaidPanicRepo) ListChannelStates(ctx context.Context, lockdownID int64) ([]RaidPanicChannelStateRow, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT lockdown_id, guild_id, channel_id, previous_slowmode_seconds
		FROM raid_panic_channel_states WHERE lockdown_id = ?`, lockdownID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RaidPanicChannelStateRow, 0)
	for rows.Next() {
		var row RaidPanicChannelStateRow
		if err := rows.Scan(&row.LockdownID, &row.GuildID, &row.ChannelID, &row.PreviousSlowmodeSeconds); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *RaidPanicRepo) EndLockdown(ctx context.Context, lockdownID int64, reason string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE raid_panic_lockdowns SET status='ended', ended_at=?, end_reason=? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), reason, lockdownID,
	)
	return err
}

func scanRaidPanicLockdown(row *sql.Row) (RaidPanicLockdownRow, bool, error) {
	var item RaidPanicLockdownRow
	var started, ends string
	var ended sql.NullString
	if err := row.Scan(&item.ID, &item.GuildID, &item.Status, &item.SlowmodeSeconds, &item.StartedBy, &started, &ends, &ended, &item.EndReason); err != nil {
		if err == sql.ErrNoRows {
			return RaidPanicLockdownRow{}, false, nil
		}
		return RaidPanicLockdownRow{}, false, err
	}
	item.StartedAt, _ = time.Parse(time.RFC3339, started)
	item.EndsAt, _ = time.Parse(time.RFC3339, ends)
	if ended.Valid && ended.String != "" {
		t, _ := time.Parse(time.RFC3339, ended.String)
		item.EndedAt = &t
	}
	return item, true, nil
}

func scanRaidPanicLockdownFromRows(rows *sql.Rows) (RaidPanicLockdownRow, error) {
	var item RaidPanicLockdownRow
	var started, ends string
	var ended sql.NullString
	if err := rows.Scan(&item.ID, &item.GuildID, &item.Status, &item.SlowmodeSeconds, &item.StartedBy, &started, &ends, &ended, &item.EndReason); err != nil {
		return RaidPanicLockdownRow{}, err
	}
	item.StartedAt, _ = time.Parse(time.RFC3339, started)
	item.EndsAt, _ = time.Parse(time.RFC3339, ends)
	if ended.Valid && ended.String != "" {
		t, _ := time.Parse(time.RFC3339, ended.String)
		item.EndedAt = &t
	}
	return item, nil
}
