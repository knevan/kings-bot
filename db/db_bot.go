package db

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() error {
	var err error
	DB, err = sql.Open("sqlite3", "db/databot.sqlite")
	if err != nil {
		log.Fatal(err)
	}

	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS tempbans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			guild_id TEXT NOT NULL,
			ban_start TEXT NOT NULL,
			ban_end TEXT,
			reason TEXT,
			banned_by TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func AddTempBan(userID, guildID, bannedBy string, duration time.Duration, reason string) error {
	banStart := time.Now().UTC().Format(time.RFC3339)
	var banEnd string
	if duration > 0 {
		banEnd = time.Now().Add(duration).UTC().Format(time.RFC3339)
	}

	_, err := DB.Exec(`
        INSERT INTO tempbans (user_id, guild_id, ban_start, ban_end, reason, banned_by)
        VALUES (?,?,?,?,?,?)
	`, userID, guildID, banStart, banEnd, reason, bannedBy)

	if err != nil {
		log.Printf("Error adding ban: %v", err)
	} else {
		log.Printf("Add ban for user %s in guild %s until %v", userID, guildID, banEnd)
	}
	return err
}

func GetExpiredBans() ([]struct {
	UserID  string
	GuildID string
}, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := DB.Query(`
        SELECT user_id, guild_id
        FROM tempbans
	    WHERE ban_end IS NOT NULL AND ban_end <= ?
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expiredBans []struct {
		UserID  string
		GuildID string
	}

	for rows.Next() {
		var ban struct {
			UserID  string
			GuildID string
		}
		if err := rows.Scan(&ban.UserID, &ban.GuildID); err != nil {
			return nil, err
		}
		expiredBans = append(expiredBans, ban)
	}
	return expiredBans, nil
}

func RemoveTempBans(userID string) error {
	_, err := DB.Exec("DELETE FROM tempbans WHERE user_id = ?", userID)
	return err
}
