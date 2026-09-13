package orchestrator

import (
	"github.com/oikos/oikos/internal/store"
)

// state.go: akses status server, wrapper tipis di atas store.DB.

func getServer(db *store.DB, id string) (store.Server, error) {
	return db.GetServer(id)
}

func setStatus(db *store.DB, id, status string) error {
	return db.UpdateServerStatus(id, status)
}
