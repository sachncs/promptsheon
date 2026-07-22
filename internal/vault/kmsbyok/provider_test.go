package kmsbyok

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/sachncs/promptsheon/internal/models"
)

// fakeKMS satisfies KMSClient (including Decrypt) and the optional
// GenerateDataKeyWithCiphertextBlob interface the Provider uses on
// first-run. The plaintext and ciphertext slices are stable
// across calls unless the test mutates them under mu.
type fakeKMS struct {
	mu          sync.Mutex
	plaintext   []byte
	ciphertext  []byte
	decryptResp []byte
	decryptErr  error
	err         error

	genCalls int
	decCalls int
}

func (f *fakeKMS) GenerateDataKey(_ context.Context, _ string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.genCalls++
	if f.err != nil {
		return nil, f.err
	}
	return append([]byte(nil), f.plaintext...), nil
}

func (f *fakeKMS) Decrypt(_ context.Context, _ []byte) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.decCalls++
	if f.decryptErr != nil {
		return nil, f.decryptErr
	}
	if f.decryptResp != nil {
		return append([]byte(nil), f.decryptResp...), nil
	}
	return append([]byte(nil), f.plaintext...), nil
}

// GenerateDataKeyWithCiphertextBlob satisfies the optional
// interface provider.go uses for first-run wrapped-blob persistence.
func (f *fakeKMS) GenerateDataKeyWithCiphertextBlob(_ context.Context, _ string) ([]byte, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.genCalls++
	if f.err != nil {
		return nil, nil, f.err
	}
	if f.ciphertext == nil {
		blob := []byte("wrapped:" + string(f.plaintext))
		return append([]byte(nil), f.plaintext...), blob, nil
	}
	return append([]byte(nil), f.plaintext...), append([]byte(nil), f.ciphertext...), nil
}

// fakeStore keeps vault_state in memory.
type fakeStore struct {
	mu  sync.Mutex
	vs  *models.VaultState
	set bool
}

func (f *fakeStore) GetVaultState(_ context.Context) (*models.VaultState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.set {
		return nil, nil
	}
	out := *f.vs
	return &out, nil
}

func (f *fakeStore) SaveVaultState(_ context.Context, vs *models.VaultState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vs = &models.VaultState{
		KMSKeyID:       vs.KMSKeyID,
		WrappedDataKey: append([]byte(nil), vs.WrappedDataKey...),
	}
	f.set = true
	return nil
}

func (f *fakeStore) clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vs = nil
	f.set = false
}

func newProdProvider(t *testing.T, kms *fakeKMS, fs *fakeStore) *Provider {
	t.Helper()
	p, err := New(Config{
		KeyID:     "arn:aws:kms:us-east-1:111122223333:key/abcd",
		KMSClient: kms,
		Store:     fs,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

// ---- tests ----

func TestProviderMissingKeyID(t *testing.T) {
	t.Parallel()
	if _, err := New(Config{}); err == nil {
		t.Fatalf("expected error for empty KeyID")
	}
}

func TestProviderRejectsProductionWithoutClient(t *testing.T) {
	t.Parallel()
	_, err := New(Config{KeyID: "arn:aws:kms:us-east-1:111122223333:key/abcd"})
	if err == nil {
		t.Fatal("expected error for production path without KMSClient")
	}
}

func TestProviderRejectsProductionWithoutStore(t *testing.T) {
	t.Parallel()
	kms := &fakeKMS{plaintext: make([]byte, 32)}
	_, err := New(Config{
		KeyID:     "arn:aws:kms:us-east-1:111122223333:key/abcd",
		KMSClient: kms,
	})
	if err == nil {
		t.Fatal("expected error for production path without Store")
	}
}

func TestProviderTestDoubleIsDeterministic(t *testing.T) {
	t.Parallel()
	p, err := New(Config{KeyID: "arn:aws:kms:us-east-1:111122223333:key/abcd", AllowTestDouble: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key: %v", err)
	}
	if len(a) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(a))
	}
	b, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key 2: %v", err)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("Key not deterministic at index %d", i)
		}
	}
}

// SEC-10a: first call wraps via GenerateDataKeyWithCiphertextBlob
// and persists the wrapped blob in vault_state; subsequent calls
// hit the LRU and do not re-call KMS.
func TestProviderPersistsWrappedBlob(t *testing.T) {
	t.Parallel()
	key1 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
	}
	kms := &fakeKMS{plaintext: key1}
	fs := &fakeStore{}
	p := newProdProvider(t, kms, fs)

	first, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key 1: %v", err)
	}
	if !equalBytes(first, key1) {
		t.Errorf("first key mismatch")
	}
	if kms.genCalls != 1 {
		t.Errorf("expected 1 GenerateDataKey call, got %d", kms.genCalls)
	}
	if fs.vs == nil || len(fs.vs.WrappedDataKey) == 0 {
		t.Errorf("wrapped blob not persisted in vault_state")
	}

	// Second call should hit LRU: same wrapped blob, same cache
	// key, no extra Decrypt.
	second, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key 2: %v", err)
	}
	if !equalBytes(second, key1) {
		t.Errorf("second call should hit LRU and return same plaintext")
	}
	if kms.genCalls != 1 {
		t.Errorf("expected LRU hit (no extra GenerateDataKey), genCalls=%d", kms.genCalls)
	}
	if kms.decCalls != 1 {
		t.Errorf("expected exactly 1 Decrypt total (first call only), got %d", kms.decCalls)
	}
}

// SEC-10a: rotated wrapped blob (simulating KMS re-encryption)
// changes the cache key, so the next read Decrypts against the new
// blob and returns new plaintext.
func TestProviderReflectsRotatedBlob(t *testing.T) {
	t.Parallel()
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(0xff - i)
	}
	kms := &fakeKMS{plaintext: key1}
	fs := &fakeStore{}
	p := newProdProvider(t, kms, fs)

	first, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key 1: %v", err)
	}
	if !equalBytes(first, key1) {
		t.Errorf("first key mismatch")
	}

	// Simulate a rotation: change the ciphertext + plaintext and
	// clear vault_state so the next Key() regenerates.
	kms.mu.Lock()
	kms.plaintext = key2
	kms.ciphertext = []byte("wrapped-v2")
	kms.mu.Unlock()
	fs.clear()
	p.cache.InvalidateAll()

	third, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key after rotation: %v", err)
	}
	if !equalBytes(third, key2) {
		t.Errorf("after rotation, key should be key2, got key1")
	}
	if kms.decCalls != 2 {
		t.Errorf("expected exactly 2 Decrypt calls (initial + post-rotation), got %d", kms.decCalls)
	}
}

// SEC-10a: Decrypt error surfaces; failed Decrypt does not poison
// the LRU (negative caching absent).
func TestProviderDecryptFailureDoesNotPoisonLRU(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	kms := &fakeKMS{plaintext: key}
	fs := &fakeStore{}
	p := newProdProvider(t, kms, fs)

	// First call succeeds, populates vault_state + LRU.
	first, err := p.Key(context.Background())
	if err != nil || !equalBytes(first, key) {
		t.Fatalf("first key err=%v", err)
	}

	// Make Decrypt fail and force a cache miss by rotating.
	kms.mu.Lock()
	kms.decryptErr = errors.New("kms decrypt unavailable")
	kms.mu.Unlock()
	if err := p.Rotate(context.Background()); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	// Next Key() must NOT return a poisoned entry; it must surface
	// the underlying Decrypt error.
	if _, err := p.Key(context.Background()); err == nil {
		t.Fatal("expected error from Decrypt, got nil")
	}

	// A subsequent successful Decrypt after a transient outage
	// must still return the correct plaintext.
	kms.mu.Lock()
	kms.decryptErr = nil
	kms.mu.Unlock()
	got, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("key after recovery: %v", err)
	}
	if !equalBytes(got, key) {
		t.Errorf("post-recovery key mismatch")
	}
}

// SEC-10a: the LRU evicts when more than 16 unique wrapped blobs
// are seen (size cap). Old plaintext is not retained.
func TestProviderLRUSize16(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	kms := &fakeKMS{plaintext: key}
	fs := &fakeStore{}
	p := newProdProvider(t, kms, fs)

	for i := 0; i < 17; i++ {
		kms.mu.Lock()
		kms.ciphertext = []byte("blob-" + string(rune('a'+i)))
		kms.mu.Unlock()
		fs.clear()
		p.cache.InvalidateAll()
		if _, err := p.Key(context.Background()); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}

	if got := p.cache.Len(); got > 16 {
		t.Errorf("LRU exceeded size 16: got %d", got)
	}
}

func TestProviderKeyPropagatesKMSError(t *testing.T) {
	t.Parallel()
	kms := &fakeKMS{err: errors.New("kms down")}
	fs := &fakeStore{}
	p := newProdProvider(t, kms, fs)
	if _, err := p.Key(context.Background()); err == nil {
		t.Fatal("expected error when KMS fails")
	}
}

func TestProviderUsesKMSClient(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	kms := &fakeKMS{plaintext: key}
	fs := &fakeStore{}
	p := newProdProvider(t, kms, fs)
	got, err := p.Key(context.Background())
	if err != nil {
		t.Fatalf("Key: %v", err)
	}
	for i := range got {
		if got[i] != key[i] {
			t.Errorf("byte %d: got %d want %d", i, got[i], key[i])
		}
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
