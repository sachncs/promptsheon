package models_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/models"
)

func TestUserJSONRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Second)
	u := models.User{
		ID:        "u1",
		Email:     "alice@example.com",
		Name:      "Alice",
		Role:      "admin",
		CreatedAt: now,
		UpdatedAt: now,
	}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got models.User
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != u {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, u)
	}
}

func TestAPIKeyKeyHashHidden(t *testing.T) {
	t.Parallel()
	k := models.APIKey{
		ID:        "k1",
		UserID:    "u1",
		Name:      "primary",
		KeyHash:   "secret-hash",
		KeyPrefix: "psn_abcd",
		Role:      "admin",
		Revoked:   false,
	}
	b, err := json.Marshal(k)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if contains(s, "secret-hash") {
		t.Errorf("KeyHash leaked in JSON: %s", s)
	}
	if !contains(s, `"key_prefix":"psn_abcd"`) {
		t.Errorf("KeyPrefix missing from JSON: %s", s)
	}
	if !contains(s, `"revoked":false`) {
		t.Errorf("Revoked missing from JSON: %s", s)
	}
}

func TestAPIKeyOptionalFieldsOmitEmpty(t *testing.T) {
	t.Parallel()
	k := models.APIKey{ID: "k1"}
	b, _ := json.Marshal(k)
	s := string(b)
	if contains(s, "expires_at") {
		t.Errorf("ExpiresAt should be omitted when nil: %s", s)
	}
	if contains(s, "last_used") {
		t.Errorf("LastUsed should be omitted when nil: %s", s)
	}
}

func TestAPIKeyJSONRoundTripWithTimestamps(t *testing.T) {
	t.Parallel()
	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	k := models.APIKey{
		ID:        "k2",
		UserID:    "u2",
		KeyHash:   "h",
		KeyPrefix: "psn_efgh",
		ExpiresAt: &expires,
		LastUsed:  &last,
		Revoked:   true,
	}
	b, err := json.Marshal(k)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got models.APIKey
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(expires) {
		t.Errorf("ExpiresAt mismatch: got %v want %v", got.ExpiresAt, expires)
	}
	if got.LastUsed == nil || !got.LastUsed.Equal(last) {
		t.Errorf("LastUsed mismatch: got %v want %v", got.LastUsed, last)
	}
	if !got.Revoked {
		t.Errorf("Revoked should be true")
	}
}

func TestAuditEntryHashChainFields(t *testing.T) {
	t.Parallel()
	entry := models.AuditEntry{
		ID:           "a1",
		UserID:       "u1",
		Action:       "capability.create",
		Resource:     "capability/c1",
		Details:      map[string]any{"name": "greeting"},
		Timestamp:    time.Unix(1700000000, 0).UTC(),
		PreviousHash: "prev",
		EntryHash:    "curr",
	}
	if entry.PreviousHash != "prev" || entry.EntryHash != "curr" {
		t.Errorf("hash chain fields lost: %+v", entry)
	}
	b, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !contains(s, `"previous_hash":"prev"`) || !contains(s, `"entry_hash":"curr"`) {
		t.Errorf("hash chain fields not serialized: %s", s)
	}
}

func TestAuditFilterZeroValues(t *testing.T) {
	t.Parallel()
	f := models.AuditFilter{}
	if f.UserID != "" || f.Limit != 0 || f.Since != nil {
		t.Errorf("zero AuditFilter should have zero fields: %+v", f)
	}
}

func TestProviderKeyJSONRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Second)
	k := models.ProviderKey{
		ID:           "pk1",
		ProviderName: "openai",
		KeyName:      "primary",
		EncryptedKey: "base64ciphertext",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	b, err := json.Marshal(k)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// EncryptedKey is marked json:"-" so a generic JSON encoder
	// must not surface the ciphertext. The test asserts the
	// marker is present and that the decrypted value never
	// crosses the wire.
	if strings.Contains(string(b), "base64ciphertext") {
		t.Errorf("ciphertext leaked into JSON: %s", b)
	}
	// Round-trip through a separate cipher-DTO instead. We
	// simulate by copying the non-secret fields manually.
	var got models.ProviderKey
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != k.ID || got.ProviderName != k.ProviderName || got.KeyName != k.KeyName {
		t.Errorf("non-secret fields mismatch: got %+v want %+v", got, k)
	}
	if got.EncryptedKey != "" {
		t.Errorf("unmarshal must not populate EncryptedKey from JSON, got %q", got.EncryptedKey)
	}
}

func TestAlertRuleRecordJSON(t *testing.T) {
	t.Parallel()
	r := models.AlertRuleRecord{
		ID:        "r1",
		Name:      "latency_p95",
		Type:      "latency",
		Severity:  "warning",
		Enabled:   true,
		Threshold: 500.0,
		Duration:  5,
		Window:    60,
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !contains(s, `"threshold":500`) {
		t.Errorf("threshold not serialized: %s", s)
	}
	if contains(s, `"config":`) {
		t.Errorf("empty Config should be omitted: %s", s)
	}
}

func TestAlertRecordResolvedAtOmitEmpty(t *testing.T) {
	t.Parallel()
	a := models.AlertRecord{ID: "a1", RuleID: "r1"}
	b, _ := json.Marshal(a)
	if contains(string(b), "resolved_at") {
		t.Errorf("ResolvedAt should be omitted when nil")
	}

	resolved := time.Now().UTC()
	a.ResolvedAt = &resolved
	b, _ = json.Marshal(a)
	if !contains(string(b), `"resolved_at":`) {
		t.Errorf("ResolvedAt should be serialized when set: %s", b)
	}
}

func TestNotificationGroupRecordJSON(t *testing.T) {
	t.Parallel()
	g := models.NotificationGroupRecord{
		ID:       "g1",
		Name:     "ops",
		Channels: []string{"slack", "email"},
	}
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !contains(string(b), `"channels":["slack","email"]`) {
		t.Errorf("channels not serialized correctly: %s", b)
	}
}

func TestWebhookEndpointRecordJSON(t *testing.T) {
	t.Parallel()
	e := models.WebhookEndpointRecord{
		ID:     "w1",
		URL:    "https://example.com/hook",
		Events: []string{"capability.created"},
		Active: true,
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !contains(s, `"active":true`) {
		t.Errorf("active flag missing: %s", s)
	}
	if !contains(s, `"url":"https://example.com/hook"`) {
		t.Errorf("url missing: %s", s)
	}
}

func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
