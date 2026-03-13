package db

import (
	"context"
	"database/sql"
	"time"
)

type ConfessionsRepo struct {
	db *sql.DB
}

type ConfessionRow struct {
	ID              int64  `json:"id"`
	GuildID         string `json:"guild_id"`
	UserID          string `json:"user_id"`
	Content         string `json:"content"`
	Status          string `json:"status"`
	PostedMessageID string `json:"posted_message_id"`
	CreatedAt       string `json:"created_at"`
	ReviewedAt      string `json:"reviewed_at"`
}

func (r *ConfessionsRepo) Create(ctx context.Context, guildID, userID, content, status string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.ExecContext(ctx, `INSERT INTO confessions(guild_id, user_id, content, status, posted_message_id, created_at, reviewed_at)
		VALUES(?, ?, ?, ?, '', ?, '')`, guildID, userID, content, status, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *ConfessionsRepo) UpdateStatus(ctx context.Context, id int64, status, postedMessageID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE confessions SET status=?, posted_message_id=?, reviewed_at=? WHERE id=?`,
		status, postedMessageID, time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (r *ConfessionsRepo) Get(ctx context.Context, id int64) (ConfessionRow, bool, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, guild_id, user_id, content, status, posted_message_id, created_at, reviewed_at FROM confessions WHERE id = ?`, id)
	var out ConfessionRow
	if err := row.Scan(&out.ID, &out.GuildID, &out.UserID, &out.Content, &out.Status, &out.PostedMessageID, &out.CreatedAt, &out.ReviewedAt); err != nil {
		if err == sql.ErrNoRows {
			return ConfessionRow{}, false, nil
		}
		return ConfessionRow{}, false, err
	}
	return out, true, nil
}

func (r *ConfessionsRepo) ListByStatus(ctx context.Context, guildID, status string, limit int) ([]ConfessionRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, guild_id, user_id, content, status, posted_message_id, created_at, reviewed_at
		FROM confessions WHERE guild_id = ? AND status = ? ORDER BY created_at DESC LIMIT ?`, guildID, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ConfessionRow, 0, limit)
	for rows.Next() {
		var row ConfessionRow
		if err := rows.Scan(&row.ID, &row.GuildID, &row.UserID, &row.Content, &row.Status, &row.PostedMessageID, &row.CreatedAt, &row.ReviewedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
