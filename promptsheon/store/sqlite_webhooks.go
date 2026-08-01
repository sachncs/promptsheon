package store

import (
	"fmt"
	"github.com/sachncs/promptsheon/promptsheon/models"
	"context"
	"database/sql"
	"strings"

	"github.com/sachncs/promptsheon/promptsheon/errs"
)

// SQLite persistence for webhooks.

func (s *SQLite) SaveWebhookEndpoint(ctx context.Context, ep *models.WebhookEndpointRecord) error {
	events := strings.Join(ep.Events, ",")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO webhook_endpoints (id, url, secret_ciphertext, events, active, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			url = excluded.url,
			secret_ciphertext = excluded.secret_ciphertext,
			events = excluded.events,
			active = excluded.active`,
		ep.ID, ep.URL, ep.SecretCiphertext, events, ep.Active, ep.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save webhook endpoint: %w", err)
	}
	return nil
}

func (s *SQLite) GetWebhookEndpoint(ctx context.Context, id string) (*models.WebhookEndpointRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, url, secret_ciphertext, events, active, created_at FROM webhook_endpoints WHERE id = ?`, id)
	ep, err := scanWebhookEndpoint(row)
	if err == sql.ErrNoRows {
		return nil, errs.ErrStoreNotFound
	}
	return ep, err
}

func (s *SQLite) DeleteWebhookEndpoint(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webhook_endpoints WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete webhook endpoint: %w", err)
	}
	return nil
}

func (s *SQLite) ListWebhookEndpoints(ctx context.Context) ([]*models.WebhookEndpointRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, secret_ciphertext, events, active, created_at FROM webhook_endpoints ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var eps []*models.WebhookEndpointRecord
	for rows.Next() {
		ep, err := scanWebhookEndpoint(rows)
		if err != nil {
			return nil, err
		}
		eps = append(eps, ep)
	}
	return eps, rows.Err()
}

func scanWebhookEndpoint(row scannable) (*models.WebhookEndpointRecord, error) {
	var ep models.WebhookEndpointRecord
	var events string
	err := row.Scan(&ep.ID, &ep.URL, &ep.SecretCiphertext, &events, &ep.Active, &ep.CreatedAt)
	if err != nil {
		return nil, err
	}
	if events != "" {
		ep.Events = strings.Split(events, ",")
	}
	return &ep, nil
}

// GetVaultState returns the singleton vault_state row, or nil if
// no wrapped data key has been persisted yet. SEC-10a.
