//go:build clickhouse

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	chgo "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/sachncs/promptsheon/internal/rollups/clickhouse"
)

// buildClickHouseWriter is the clickhouse-tagged replacement for
// the placeholder in main.go. When the binary is built with
// `-tags clickhouse`, this function constructs a real
// clickhouse.Writer using the operator-supplied DSN; without
// the tag, main.go's stub returns the "not compiled in" error
// and the daemon boots cleanly.
func buildClickHouseWriter(ctx context.Context, dsn, database string, logger *slog.Logger) (any, error) {
	addr, db, err := parseClickHouseDSN(dsn, database)
	if err != nil {
		return nil, err
	}
	opt := &chgo.Options{
		Addr: []string{addr},
		Auth: chgo.Auth{Database: db},
	}
	w, err := clickhouse.New(ctx, opt, db)
	if err != nil {
		if logger != nil {
			logger.Error("clickhouse: connect failed", "err", err)
		}
		return nil, err
	}
	if logger != nil {
		logger.Info("clickhouse: writer ready", "addr", addr, "database", db)
	}
	return w, nil
}

// parseClickHouseDSN returns the host:port + database from the
// operator's DSN. Accepts two shapes:
//
//	clickhouse://user:pass@host:9000/database
//	clickhouse://host:9000/database
//
// When the DSN doesn't carry a database, the caller-supplied
// database fallback is used. Anything else returns an error so
// the daemon fails to start instead of writing to the wrong DB.
func parseClickHouseDSN(dsn, fallbackDB string) (addr, db string, err error) {
	u, perr := url.Parse(dsn)
	if perr != nil {
		return "", "", perr
	}
	if u.Scheme != "clickhouse" && u.Scheme != "https" {
		return "", "", errDSNScheme
	}
	addr = u.Host
	if addr == "" {
		return "", "", errDSNNoHost
	}
	db = strings.TrimPrefix(u.Path, "/")
	if db == "" {
		db = fallbackDB
	}
	if db == "" {
		return "", "", errDSNNoDB
	}
	return addr, db, nil
}

// errDSN* are package-local sentinels so callers can distinguish
// configuration errors from connection errors. The placeholder
// in main.go wraps these via fmt.Errorf so the daemon logs them
// with the same shape as before.
var (
	errDSNScheme = errors.New("clickhouse: DSN must use clickhouse:// or https:// scheme")
	errDSNNoHost = errors.New("clickhouse: DSN missing host")
	errDSNNoDB   = errors.New("clickhouse: DSN missing database (and no fallback)")
)

// _ keeps the fmt import used (the codebase commonly errors-via-fmt).
var _ = fmt.Sprintf
