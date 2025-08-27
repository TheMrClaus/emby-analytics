package tasks

import (
	"database/sql"
	"log"
	"time"

	"emby-analytics/internal/config"
	"emby-analytics/internal/emby"
)

// StartUserSyncLoop now ONLY runs the periodic task. The initial run is in main.go.
func StartUserSyncLoop(db *sql.DB, em *emby.Client, cfg config.Config) {
	if cfg.UserSyncIntervalSec <= 0 {
		log.Println("[usersync] periodic sync disabled (interval <= 0)")
		return
	}

	log.Printf("[usersync] starting periodic loop with interval %d seconds (%d hours)",
		cfg.UserSyncIntervalSec, cfg.UserSyncIntervalSec/3600)

	ticker := time.NewTicker(time.Duration(cfg.UserSyncIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		runUserSync(db, em)
	}
}

// runUserSync is now private and unchanged.
func runUserSync(db *sql.DB, em *emby.Client) {
	log.Println("[usersync] starting user sync...")
	startTime := time.Now()
	apiCalls := 0

	users, err := em.GetUsers()
	apiCalls++

	if err != nil {
		log.Printf("[usersync] ERROR fetching users from Emby: %v", err)
		return
	}

	if len(users) == 0 {
		log.Println("[usersync] WARNING: Emby returned 0 users")
		return
	}

	log.Printf("[usersync] found %d users from Emby", len(users))

	upserted := 0
	for _, user := range users {
		result, err := db.Exec(`INSERT INTO emby_user (id, name) VALUES (?, ?)
		                   ON CONFLICT(id) DO UPDATE SET name=excluded.name`,
			user.Id, user.Name)
		if err != nil {
			log.Printf("[usersync] ERROR upserting user %s (%s): %v", user.Name, user.Id, err)
			continue
		}

		rows, _ := result.RowsAffected()
		if rows > 0 {
			upserted++
		}

		apiCalls += syncUserWatchData(db, em, user.Id, user.Name)
	}

	var totalInDB int
	_ = db.QueryRow(`SELECT COUNT(*) FROM emby_user`).Scan(&totalInDB)

	duration := time.Since(startTime)
	log.Printf("[usersync] completed in %v: %d API calls, upserted %d users, total in DB: %d",
		duration.Round(time.Millisecond), apiCalls, upserted, totalInDB)
}

func syncUserWatchData(db *sql.DB, em *emby.Client, userID, userName string) int {
	userDataItems, err := em.GetUserData(userID)
	apiCalls := 1

	if err != nil {
		log.Printf("[usersync] failed to get user data for %s: %v", userName, err)
		return apiCalls
	}

	var totalWatchMs int64
	for _, item := range userDataItems {
		if item.UserData.Played && item.RunTimeTicks > 0 {
			totalWatchMs += item.RunTimeTicks / 10000
		}
	}

	_, err = db.Exec(`INSERT INTO lifetime_watch (user_id, total_ms)
	                  VALUES (?, ?)
	                  ON CONFLICT(user_id) DO UPDATE SET total_ms = excluded.total_ms`,
		userID, totalWatchMs)

	if err != nil {
		log.Printf("[usersync] failed to update lifetime watch for %s: %v", userName, err)
	}

	return apiCalls
}

// RunUserSyncOnce is now the primary way to trigger a sync.
func RunUserSyncOnce(db *sql.DB, em *emby.Client) {
	runUserSync(db, em)
}
