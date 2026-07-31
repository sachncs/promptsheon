package store
import (
	"context"
	"database/sql"
	"fmt"

	"github.com/sachncs/promptsheon/backend/errs"
	"github.com/sachncs/promptsheon/backend/models"
)

// SQLite persistence for providerkeys.

func (s *SQLite) SaveProviderKey(ctx context.Context, pk *models.ProviderKey) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO provider_keys (id, provider_name, key_name, encrypted_key, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET encrypted_key=excluded.encrypted_key, updated_at=excluded.updated_at`,
		pk.ID, pk.ProviderName, pk.KeyName, pk.EncryptedKey, pk.CreatedAt, pk.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save provider key: %w", err)
	}
	return nil
}


func (s *SQLite) GetProviderKey(ctx context.Context, id string) (*models.ProviderKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, provider_name, key_name, encrypted_key, created_at, updated_at
		 FROM provider_keys WHERE id = ?`, id,
	)
	return scanProviderKey(row)
}


func (s *SQLite) GetProviderKeyByName(ctx context.Context, providerName, keyName string) (*models.ProviderKey, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, provider_name, key_name, encrypted_key, created_at, updated_at
		 FROM provider_keys WHERE provider_name = ? AND key_name = ?`, providerName, keyName,
	)
	return scanProviderKey(row)
}


func (s *SQLite) DeleteProviderKey(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM provider_keys WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete provider key: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("provider key not found: %s", id)
	}
	return nil
}


func (s *SQLite) ListProviderKeys(ctx context.Context) ([]*models.ProviderKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, provider_name, key_name, encrypted_key, created_at, updated_at
		 FROM provider_keys ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list provider keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var keys []*models.ProviderKey
	for rows.Next() {
		k, err := scanProviderKeyRow(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}


func scanProviderKey(row scannable) (*models.ProviderKey, error) {
	var pk models.ProviderKey
	err := row.Scan(&pk.ID, &pk.ProviderName, &pk.KeyName, &pk.EncryptedKey, &pk.CreatedAt, &pk.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errs.ErrStoreNotFound
		}
		return nil, fmt.Errorf("scan provider key: %w", err)
	}
	return &pk, nil
}


func scanProviderKeyRow(rows *sql.Rows) (*models.ProviderKey, error) {
	return scanProviderKey(rows)
}

// Alert Rules


