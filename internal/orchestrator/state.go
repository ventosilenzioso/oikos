package orchestrator

import (
	"github.com/oikos/oikos/internal/store"
	"time"
)

// state.go: akses status server, wrapper tipis di atas store.DB.

func getServer(db *store.DB, id string) (store.Server, error) {
	return db.GetServer(id)
}

func (l *Lifecycle) setLastCrash(id string) error {
	return l.db.SetLastCrash(id, time.Now().UTC())
}

func setStatus(db *store.DB, id, status string) error {
	return db.UpdateServerStatus(id, status)
}
