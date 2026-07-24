//go:build !clickhouse

package main

import (
	"context"
	"fmt"
	"log/slog"
)

// buildClickHouseWriter in the !clickhouse build returns the
// "not compiled in" sentinel. The clickhouse-tagged build
// (main_clickhouse.go) provides a real implementation that
// constructs a clickhouse.Writer from the operator's DSN.
func buildClickHouseWriter(ctx context.Context, dsn, database string, logger *slog.Logger) (any, error) {
	return nil, fmt.Errorf("clickhouse writer not compiled in (rebuild with -tags clickhouse)")
}
