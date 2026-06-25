package api

import (
	"net/http"
	"time"

	"github.com/sachn-cs/promptsheon/internal/models"
)

func (s *Server) handleSaveVaultKey(w http.ResponseWriter, r *http.Request) error {
	if s.vault == nil {
		return badRequest("vault not configured")
	}

	var req struct {
		ProviderName string `json:"provider_name"`
		KeyName      string `json:"key_name"`
		PlaintextKey string `json:"key"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.ProviderName == "" || req.KeyName == "" || req.PlaintextKey == "" {
		return ErrBadRequest
	}

	encrypted, err := s.vault.Encrypt(req.PlaintextKey)
	if err != nil {
		return err
	}

	now := time.Now()
	pk := &models.ProviderKey{
		ID:           generateID(),
		ProviderName: req.ProviderName,
		KeyName:      req.KeyName,
		EncryptedKey: encrypted,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.db.SaveProviderKey(r.Context(), pk); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "vault_key:"+pk.ID, map[string]any{"provider": pk.ProviderName, "key_name": pk.KeyName})

	// Return without the encrypted key for security
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":            pk.ID,
		"provider_name": pk.ProviderName,
		"key_name":      pk.KeyName,
		"created_at":    pk.CreatedAt,
	})
	return nil
}

func (s *Server) handleListVaultKeys(w http.ResponseWriter, r *http.Request) error {
	keys, err := s.db.ListProviderKeys(r.Context())
	if err != nil {
		return err
	}
	if keys == nil {
		keys = []*models.ProviderKey{}
	}

	// Strip encrypted keys from response
	type keyInfo struct {
		ID           string    `json:"id"`
		ProviderName string    `json:"provider_name"`
		KeyName      string    `json:"key_name"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
	}
	result := make([]keyInfo, len(keys))
	for i, k := range keys {
		result[i] = keyInfo{
			ID:           k.ID,
			ProviderName: k.ProviderName,
			KeyName:      k.KeyName,
			CreatedAt:    k.CreatedAt,
			UpdatedAt:    k.UpdatedAt,
		}
	}
	writeJSON(w, http.StatusOK, result)
	return nil
}

func (s *Server) handleDeleteVaultKey(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if err := s.db.DeleteProviderKey(r.Context(), id); err != nil {
		return ErrNotFound
	}
	s.audit(r.Context(), "delete", "vault_key:"+id, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
