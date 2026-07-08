package snapshot

import (
	"database/sql"

	_ "modernc.org/sqlite" // sqlite driver
)

func newTestDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	return db, nil
}
