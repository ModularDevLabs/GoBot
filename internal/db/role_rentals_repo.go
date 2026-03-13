package db

import (
	"context"
	"database/sql"
	"time"
)

type RoleRentalsRepo struct {
	db *sql.DB
}

type RoleRentalRow struct {
	ID        int64  `json:"id"`
	GuildID   string `json:"guild_id"`
	UserID    string `json:"user_id"`
	RoleID    string `json:"role_id"`
	StartedAt string `json:"started_at"`
	ExpiresAt string `json:"expires_at"`
	Status    string `json:"status"`
}

func (r *RoleRentalsRepo) Create(ctx context.Context, guildID, userID, roleID string, durationMinutes int) error {
	now := time.Now().UTC()
	exp := now.Add(time.Duration(durationMinutes) * time.Minute)
	_, err := r.db.ExecContext(ctx, `INSERT INTO role_rentals(guild_id, user_id, role_id, started_at, expires_at, status)
		VALUES(?, ?, ?, ?, ?, 'active')`,
		guildID, userID, roleID, now.Format(time.RFC3339), exp.Format(time.RFC3339),
	)
	return err
}

func (r *RoleRentalsRepo) Due(ctx context.Context, now time.Time, limit int) ([]RoleRentalRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, guild_id, user_id, role_id, started_at, expires_at, status
		FROM role_rentals WHERE status='active' AND expires_at <= ? ORDER BY expires_at ASC LIMIT ?`,
		now.UTC().Format(time.RFC3339), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RoleRentalRow, 0, limit)
	for rows.Next() {
		var row RoleRentalRow
		if err := rows.Scan(&row.ID, &row.GuildID, &row.UserID, &row.RoleID, &row.StartedAt, &row.ExpiresAt, &row.Status); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *RoleRentalsRepo) MarkExpired(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE role_rentals SET status='expired' WHERE id = ?`, id)
	return err
}
