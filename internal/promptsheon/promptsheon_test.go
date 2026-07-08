package promptsheon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func setupTestRepo(t *testing.T) {
	t.Helper()
	_ = os.RemoveAll(PromptsheonDir)
	if err := Init(); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
}

func teardownTestRepo(t *testing.T) {
	t.Helper()
	_ = os.RemoveAll(PromptsheonDir)
}

// ---------------------------------------------------------------------------
// Repository initialization
// ---------------------------------------------------------------------------

func TestInit(t *testing.T) {
	teardownTestRepo(t)
	defer teardownTestRepo(t)

	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	if !IsInitialized() {
		t.Fatal("IsInitialized() should be true after Init()")
	}

	dirs := []string{
		filepath.Join(PromptsheonDir, objectsDir),
		filepath.Join(PromptsheonDir, headsDir),
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			t.Fatalf("expected directory %s to exist", d)
		}
	}

	head, err := ReadHEAD()
	if err != nil {
		t.Fatalf("ReadHEAD() error: %v", err)
	}
	if head != "ref: refs/heads/main" {
		t.Fatalf("expected HEAD to be 'ref: refs/heads/main', got %q", head)
	}
}

func TestInitIdempotent(t *testing.T) {
	teardownTestRepo(t)
	defer teardownTestRepo(t)

	if err := Init(); err != nil {
		t.Fatalf("first Init() failed: %v", err)
	}
	// Second Init should succeed (it's idempotent for directories and HEAD).
	if err := Init(); err != nil {
		t.Fatalf("second Init() failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Blob operations
// ---------------------------------------------------------------------------

func TestBlobRoundTrip(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	data := "You are a helpful AI assistant."
	obj := NewBlobObject(data)
	hash, err := ObjectHash(obj)
	if err != nil {
		t.Fatalf("ObjectHash: %v", err)
	}

	if len(hash) != 64 {
		t.Fatalf("expected 64-char hash, got %d: %s", len(hash), hash)
	}

	writtenHash, err := WriteObject(obj)
	if err != nil {
		t.Fatalf("WriteObject() error: %v", err)
	}
	if writtenHash != hash {
		t.Fatalf("hash mismatch: %s vs %s", hash, writtenHash)
	}

	readObj, err := ReadObject(hash)
	if err != nil {
		t.Fatalf("ReadObject() error: %v", err)
	}
	if readObj.Data != data {
		t.Fatalf("data mismatch: expected %q, got %q", data, readObj.Data)
	}

	if !ObjectExists(hash) {
		t.Fatal("ObjectExists() should be true for existing object")
	}
}

func TestEmptyBlob(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	obj := NewBlobObject("")
	hash, err := WriteObject(obj)
	if err != nil {
		t.Fatalf("WriteObject() empty blob: %v", err)
	}
	if len(hash) != 64 {
		t.Fatalf("expected 64-char hash, got %d", len(hash))
	}

	readObj, err := ReadObject(hash)
	if err != nil {
		t.Fatalf("ReadObject() empty blob: %v", err)
	}
	if readObj.Data != "" {
		t.Fatalf("expected empty data, got %q", readObj.Data)
	}
}

func TestBlobDeduplication(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	data := "deduplicated content"
	obj := NewBlobObject(data)

	hash1, err := WriteObject(obj)
	if err != nil {
		t.Fatalf("first WriteObject(): %v", err)
	}

	hash2, err := WriteObject(obj)
	if err != nil {
		t.Fatalf("second WriteObject(): %v", err)
	}

	if hash1 != hash2 {
		t.Fatal("identical content must produce identical hashes")
	}
}

func TestLargeBlob(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	data := strings.Repeat("A", 100000)
	obj := NewBlobObject(data)
	hash, err := WriteObject(obj)
	if err != nil {
		t.Fatalf("WriteObject() large blob: %v", err)
	}

	readObj, err := ReadObject(hash)
	if err != nil {
		t.Fatalf("ReadObject() large blob: %v", err)
	}
	if len(readObj.Data) != 100000 {
		t.Fatalf("expected 100000 bytes, got %d", len(readObj.Data))
	}
}

// ---------------------------------------------------------------------------
// Object immutability
// ---------------------------------------------------------------------------

func TestObjectImmutability(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	obj := NewBlobObject("immutable data")
	hash, err := WriteObject(obj)
	if err != nil {
		t.Fatalf("WriteObject() error: %v", err)
	}

	objPath := filepath.Join(PromptsheonDir, objectsDir, hash[:2], hash[2:])
	info, err := os.Stat(objPath)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0444 {
		t.Fatalf("expected 0444 permissions, got %o", perm)
	}

	if err := os.WriteFile(objPath, []byte("tampered"), 0444); err == nil {
		t.Fatal("expected write to read-only file to fail")
	}
}

// ---------------------------------------------------------------------------
// Object size
// ---------------------------------------------------------------------------

func TestObjectFileSize(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	obj := NewBlobObject("size test content")
	hash, err := WriteObject(obj)
	if err != nil {
		t.Fatalf("WriteObject(): %v", err)
	}

	size, err := ObjectFileSize(hash)
	if err != nil {
		t.Fatalf("ObjectFileSize(): %v", err)
	}
	if size <= 0 {
		t.Fatalf("expected positive file size, got %d", size)
	}

	_, err = ObjectFileSize(strings.Repeat("a", 64))
	if err == nil {
		t.Fatal("expected error for non-existent object")
	}
}

// ---------------------------------------------------------------------------
// Tree operations
// ---------------------------------------------------------------------------

func TestTreeDeterminism(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b1 := NewBlobObject("prompt A")
	b2 := NewBlobObject("prompt B")
	h1, _ := WriteObject(b1)
	h2, _ := WriteObject(b2)

	entries1 := []TreeEntry{
		{Name: "prompt-b", Type: TypeBlob, Hash: h2},
		{Name: "prompt-a", Type: TypeBlob, Hash: h1},
	}
	t1 := NewTreeObject(entries1)
	hash1, err := ObjectHash(t1)
	if err != nil {
		t.Fatalf("ObjectHash: %v", err)
	}

	entries2 := []TreeEntry{
		{Name: "prompt-a", Type: TypeBlob, Hash: h1},
		{Name: "prompt-b", Type: TypeBlob, Hash: h2},
	}
	t2 := NewTreeObject(entries2)
	hash2, err := ObjectHash(t2)
	if err != nil {
		t.Fatalf("ObjectHash: %v", err)
	}

	if hash1 != hash2 {
		t.Fatalf("trees with same entries in different order should hash identically: %s vs %s", hash1, hash2)
	}
}

func TestEmptyTree(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	tree := NewTreeObject(nil)
	hash, err := WriteObject(tree)
	if err != nil {
		t.Fatalf("WriteObject() empty tree: %v", err)
	}

	readObj, err := ReadObject(hash)
	if err != nil {
		t.Fatalf("ReadObject() empty tree: %v", err)
	}
	if len(readObj.Entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(readObj.Entries))
	}
}

func TestNestedTrees(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	leaf1 := NewBlobObject("leaf1")
	leaf2 := NewBlobObject("leaf2")
	h1, _ := WriteObject(leaf1)
	h2, _ := WriteObject(leaf2)

	subTree := NewTreeObject([]TreeEntry{
		{Name: "leaf-a", Type: TypeBlob, Hash: h1},
		{Name: "leaf-b", Type: TypeBlob, Hash: h2},
	})
	subHash, err := WriteObject(subTree)
	if err != nil {
		t.Fatalf("WriteObject() subtree: %v", err)
	}

	root := NewTreeObject([]TreeEntry{
		{Name: "subtree", Type: TypeTree, Hash: subHash},
	})
	rootHash, err := WriteObject(root)
	if err != nil {
		t.Fatalf("WriteObject() root tree: %v", err)
	}

	readRoot, err := ReadObject(rootHash)
	if err != nil {
		t.Fatalf("ReadObject() root tree: %v", err)
	}
	if len(readRoot.Entries) != 1 {
		t.Fatalf("expected 1 root entry, got %d", len(readRoot.Entries))
	}
	if readRoot.Entries[0].Type != TypeTree {
		t.Fatalf("expected tree type, got %s", readRoot.Entries[0].Type)
	}
}

// ---------------------------------------------------------------------------
// Commit and log
// ---------------------------------------------------------------------------

func TestCommitAndLog(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b1 := NewBlobObject("system prompt")
	b2 := NewBlobObject("tool spec")
	h1, _ := WriteObject(b1)
	h2, _ := WriteObject(b2)

	tree := NewTreeObject([]TreeEntry{
		{Name: "system-prompt", Type: TypeBlob, Hash: h1},
		{Name: "tool-spec", Type: TypeBlob, Hash: h2},
	})
	treeHash, err := WriteObject(tree)
	if err != nil {
		t.Fatalf("WriteObject(tree) error: %v", err)
	}

	telemetry := map[string]any{"accuracy": 0.95, "latency_ms": 200}
	result, err := Commit(treeHash, nil, "test-author", "initial commit", telemetry)
	if err != nil {
		t.Fatalf("Commit() error: %v", err)
	}
	if result.Hash == "" {
		t.Fatal("commit hash should not be empty")
	}
	if result.Ref != "main" {
		t.Fatalf("expected ref 'main', got %q", result.Ref)
	}

	commits, err := Log(10)
	if err != nil {
		t.Fatalf("Log() error: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if commits[0].Message != "initial commit" {
		t.Fatalf("message mismatch: %s", commits[0].Message)
	}
	if commits[0].Author != "test-author" {
		t.Fatalf("author mismatch: %s", commits[0].Author)
	}
}

func TestCommitNoParent(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b := NewBlobObject("root")
	bh, _ := WriteObject(b)
	th, _ := WriteObject(NewTreeObject([]TreeEntry{{Name: "root", Type: TypeBlob, Hash: bh}}))

	_, err := Commit(th, nil, "author", "root commit", nil)
	if err != nil {
		t.Fatalf("Commit() root: %v", err)
	}

	commits, err := Log(10)
	if err != nil {
		t.Fatalf("Log(): %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if len(commits[0].Parents) != 0 {
		t.Fatalf("expected 0 parents for root commit, got %d", len(commits[0].Parents))
	}
}

func TestMergeCommit(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	rootBlob := NewBlobObject("root")
	rootHash, _ := WriteObject(rootBlob)
	rootTree, _ := WriteObject(NewTreeObject([]TreeEntry{{Name: "root", Type: TypeBlob, Hash: rootHash}}))

	main1, _ := Commit(rootTree, nil, "author", "main root", nil)
	if err := Checkout(main1.Hash); err != nil {
		t.Fatalf("Checkout detached: %v", err)
	}

	feature, _ := Commit(rootTree, []string{main1.Hash}, "author", "feature commit", nil)

	if err := Checkout("main"); err != nil {
		t.Fatalf("Checkout main: %v", err)
	}
	main2, _ := Commit(rootTree, []string{main1.Hash}, "author", "main second", nil)

	mergeCommit, err := Commit(rootTree, []string{main2.Hash, feature.Hash}, "author", "merge", nil)
	if err != nil {
		t.Fatalf("Commit() merge: %v", err)
	}

	commits, err := Log(10)
	if err != nil {
		t.Fatalf("Log(): %v", err)
	}

	// The merge commit should be first, and it should have 2 parents.
	if len(commits) == 0 {
		t.Fatal("expected at least 1 commit")
	}
	if commits[0].Hash != mergeCommit.Hash {
		t.Fatalf("expected merge commit first, got %s", commits[0].Hash)
	}
	if len(commits[0].Parents) != 2 {
		t.Fatalf("expected 2 parents for merge commit, got %d", len(commits[0].Parents))
	}
}

// ---------------------------------------------------------------------------
// Branch operations
// ---------------------------------------------------------------------------

func TestBranchAndCheckout(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	tree := NewBlobObject("root tree")
	treeHash, _ := WriteObject(tree)
	result1, _ := Commit(treeHash, nil, "author", "first commit", nil)

	if err := CreateBranch("feature", ""); err != nil {
		t.Fatalf("CreateBranch() error: %v", err)
	}

	if err := Checkout("feature"); err != nil {
		t.Fatalf("Checkout('feature') error: %v", err)
	}

	ref, err := GetCurrentRef()
	if err != nil {
		t.Fatalf("GetCurrentRef() error: %v", err)
	}
	if ref != "feature" {
		t.Fatalf("expected 'feature', got %q", ref)
	}

	result2, _ := Commit(treeHash, []string{result1.Hash}, "author", "feature commit", nil)
	if result2.Ref != "feature" {
		t.Fatalf("expected ref 'feature', got %q", result2.Ref)
	}

	if e := Checkout("main"); e != nil {
		t.Fatalf("Checkout('main') error: %v", e)
	}

	commits, _ := Log(10)
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit on main, got %d", len(commits))
	}
	if commits[0].Message != "first commit" {
		t.Fatalf("expected 'first commit', got %q", commits[0].Message)
	}

	refs, err := ListRefs()
	if err != nil {
		t.Fatalf("ListRefs() error: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d: %v", len(refs), refs)
	}
}

func TestBranchDelete(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	tree := NewBlobObject("data")
	treeHash, _ := WriteObject(tree)
	_, _ = Commit(treeHash, nil, "author", "init", nil)

	if err := CreateBranch("feature", ""); err != nil {
		t.Fatalf("CreateBranch(): %v", err)
	}
	refs, _ := ListRefs()
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs before delete, got %d", len(refs))
	}

	if err := DeleteBranch("feature"); err != nil {
		t.Fatalf("DeleteBranch(): %v", err)
	}
	refs, _ = ListRefs()
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref after delete, got %d", len(refs))
	}
}

func TestBranchDeleteErrors(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	tree := NewBlobObject("data")
	treeHash, _ := WriteObject(tree)
	_, _ = Commit(treeHash, nil, "author", "init", nil)

	if err := DeleteBranch("nonexistent"); err == nil {
		t.Fatal("expected error deleting nonexistent branch")
	}

	if err := DeleteBranch("main"); err == nil {
		t.Fatal("expected error deleting current branch")
	}
}

func TestBranchInvalidName(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	invalidNames := []string{"", "has space", "has\tcontrol", "has/ slash", "has\\backslash"}
	for _, name := range invalidNames {
		if err := CreateBranch(name, ""); err == nil {
			t.Fatalf("expected error for invalid branch name %q", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Detached HEAD
// ---------------------------------------------------------------------------

func TestDetachedHEAD(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	tree := NewBlobObject("detached test")
	treeHash, _ := WriteObject(tree)

	result, err := Commit(treeHash, nil, "author", "detached commit", nil)
	if err != nil {
		t.Fatalf("Commit() error: %v", err)
	}

	if e := Checkout(result.Hash); e != nil {
		t.Fatalf("Checkout() error: %v", e)
	}

	head, _ := ReadHEAD()
	if !IsHEADDetached(head) {
		t.Fatal("expected detached HEAD state")
	}

	ref, err := GetCurrentRef()
	if err != nil {
		t.Fatalf("GetCurrentRef() error: %v", err)
	}
	if ref != "" {
		t.Fatalf("expected empty ref for detached HEAD, got %q", ref)
	}
}

func TestDetachedHeadCommit(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	tree := NewBlobObject("data")
	treeHash, _ := WriteObject(tree)

	c1, _ := Commit(treeHash, nil, "author", "first", nil)
	_ = Checkout(c1.Hash)

	c2, err := Commit(treeHash, []string{c1.Hash}, "author", "detached commit", nil)
	if err != nil {
		t.Fatalf("Commit() on detached HEAD: %v", err)
	}
	if c2.Ref != "" {
		t.Fatalf("expected empty ref for detached HEAD commit, got %q", c2.Ref)
	}

	rootHash, err := GetCurrentCommitHash()
	if err != nil {
		t.Fatalf("GetCurrentCommitHash(): %v", err)
	}
	if rootHash != c2.Hash {
		t.Fatalf("expected %s, got %s", c2.Hash, rootHash)
	}
}

func TestReAttachHEAD(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	tree := NewBlobObject("data")
	treeHash, _ := WriteObject(tree)
	c1, _ := Commit(treeHash, nil, "author", "first", nil)
	_ = Checkout(c1.Hash)

	if err := CreateBranch("from-detached", ""); err != nil {
		t.Fatalf("CreateBranch() from detached HEAD: %v", err)
	}

	ref, err := GetCurrentRef()
	if err != nil {
		t.Fatalf("GetCurrentRef(): %v", err)
	}
	if ref != "" {
		t.Fatalf("expected detached HEAD still, got %q", ref)
	}

	if err := Checkout("from-detached"); err != nil {
		t.Fatalf("Checkout('from-detached'): %v", err)
	}
	ref, _ = GetCurrentRef()
	if ref != "from-detached" {
		t.Fatalf("expected 'from-detached', got %q", ref)
	}
}

// ---------------------------------------------------------------------------
// Log edge cases
// ---------------------------------------------------------------------------

func TestEmptyRepoLog(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	commits, err := Log(10)
	if err != nil {
		t.Fatalf("Log() error on empty repo: %v", err)
	}
	if commits != nil {
		t.Fatal("expected nil commits for empty repo")
	}
}

func TestLogWithMergeCommits(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	rootBlob := NewBlobObject("root")
	rootHash, _ := WriteObject(rootBlob)
	rootTree, _ := WriteObject(NewTreeObject([]TreeEntry{{Name: "root", Type: TypeBlob, Hash: rootHash}}))

	c1, _ := Commit(rootTree, nil, "author", "root", nil)
	_ = Checkout(c1.Hash)
	c2, _ := Commit(rootTree, []string{c1.Hash}, "author", "branch", nil)
	_ = Checkout("main")
	c3, _ := Commit(rootTree, []string{c1.Hash}, "author", "main second", nil)
	_, _ = Commit(rootTree, []string{c3.Hash, c2.Hash}, "author", "merge", nil)

	commits, err := Log(10)
	if err != nil {
		t.Fatalf("Log(): %v", err)
	}

	visited := make(map[string]bool)
	for _, c := range commits {
		if visited[c.Hash] {
			t.Fatalf("duplicate commit in log: %s", c.Hash)
		}
		visited[c.Hash] = true
	}
}

// ---------------------------------------------------------------------------
// Diff operations
// ---------------------------------------------------------------------------

func TestDiffIntelligence(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	prompt1 := NewBlobObject("You are a helpful assistant.")
	prompt2 := NewBlobObject("You are an expert AI assistant.")
	h1, _ := WriteObject(prompt1)
	h2, _ := WriteObject(prompt2)

	tree1 := NewTreeObject([]TreeEntry{
		{Name: "system-prompt", Type: TypeBlob, Hash: h1},
	})
	tree2 := NewTreeObject([]TreeEntry{
		{Name: "system-prompt", Type: TypeBlob, Hash: h2},
	})
	th1, _ := WriteObject(tree1)
	th2, _ := WriteObject(tree2)

	tel1 := map[string]any{"accuracy": 0.90, "latency_ms": 100}
	tel2 := map[string]any{"accuracy": 0.95, "latency_ms": 150}

	c1, _ := Commit(th1, nil, "author", "v1", tel1)
	c2, _ := Commit(th2, []string{c1.Hash}, "author", "v2", tel2)

	diff, err := DiffIntelligence(c1.Hash, c2.Hash)
	if err != nil {
		t.Fatalf("DiffIntelligence() error: %v", err)
	}

	if diff.CommitA != c1.Hash || diff.CommitB != c2.Hash {
		t.Fatal("commit hashes in diff result mismatch")
	}

	hasTextDiff := false
	hasMetricDiff := false
	for _, de := range diff.TreeDiffs {
		if de.TextDiff != "" {
			hasTextDiff = true
		}
		if len(de.MetricDiffs) > 0 {
			hasMetricDiff = true
		}
	}

	if !hasTextDiff {
		t.Fatal("expected text diff for modified prompt")
	}
	if !hasMetricDiff {
		t.Fatal("expected metric diff for telemetry changes")
	}
}

func TestDiffIdenticalCommits(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b := NewBlobObject("same content")
	bh, _ := WriteObject(b)
	th, _ := WriteObject(NewTreeObject([]TreeEntry{{Name: "file", Type: TypeBlob, Hash: bh}}))

	c, _ := Commit(th, nil, "author", "single", nil)
	diff, err := DiffIntelligence(c.Hash, c.Hash)
	if err != nil {
		t.Fatalf("DiffIntelligence() self: %v", err)
	}
	if len(diff.TreeDiffs) != 0 {
		t.Fatalf("expected 0 diffs for identical commits, got %d", len(diff.TreeDiffs))
	}
}

// ---------------------------------------------------------------------------
// Determinism
// ---------------------------------------------------------------------------

func TestCommitDeterminism(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	tree := NewBlobObject("deterministic test")
	treeHash, _ := WriteObject(tree)

	tel := map[string]any{"z_score": 0.5, "a_score": 0.9}
	c := NewCommitObject(treeHash, nil, "author", "msg", tel)
	hash1, err := ObjectHash(c)
	if err != nil {
		t.Fatalf("ObjectHash: %v", err)
	}
	hash2, err := ObjectHash(c)
	if err != nil {
		t.Fatalf("ObjectHash: %v", err)
	}

	if hash1 != hash2 {
		t.Fatal("hashing the same object twice must produce the same hash")
	}

	serialized, err := canonicalSerialize(c)
	if err != nil {
		t.Fatalf("canonicalSerialize() error: %v", err)
	}

	var deserialized Object
	if e := json.Unmarshal(serialized, &deserialized); e != nil {
		t.Fatalf("Unmarshal() error: %v", e)
	}

	hash3, err := ObjectHash(&deserialized)
	if err != nil {
		t.Fatalf("ObjectHash: %v", err)
	}
	if hash1 != hash3 {
		t.Fatal("serialize-deserialize round trip must preserve hash")
	}
}

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

func TestObjectNotFound(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	_, err := ReadObject("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err == nil {
		t.Fatal("expected error for non-existent object")
	}
}

func TestInvalidHash(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	_, err := ReadObject("short")
	if err == nil {
		t.Fatal("expected error for invalid hash")
	}
}

func TestUninitializedRepoErrors(t *testing.T) {
	_ = os.RemoveAll(PromptsheonDir)
	defer func() { _ = os.RemoveAll(PromptsheonDir) }()

	if IsInitialized() {
		t.Fatal("expected IsInitialized() to be false")
	}

	_, err := Commit("a", nil, "", "", nil)
	if err != ErrRepoNotInitialized {
		t.Fatalf("expected ErrRepoNotInitialized, got %v", err)
	}

	_, err = Log(10)
	if err != ErrRepoNotInitialized {
		t.Fatalf("expected ErrRepoNotInitialized, got %v", err)
	}

	if err := CreateBranch("x", ""); err != ErrRepoNotInitialized {
		t.Fatalf("expected ErrRepoNotInitialized, got %v", err)
	}

	if err := DeleteBranch("x"); err != ErrRepoNotInitialized {
		t.Fatalf("expected ErrRepoNotInitialized, got %v", err)
	}

	if err := Checkout("main"); err != ErrRepoNotInitialized {
		t.Fatalf("expected ErrRepoNotInitialized, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Ref operations
// ---------------------------------------------------------------------------

func TestRefReadWrite(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	hash := strings.Repeat("a", 64)
	if err := WriteRef("test-ref", hash); err != nil {
		t.Fatalf("WriteRef(): %v", err)
	}

	read, err := ReadRef("test-ref")
	if err != nil {
		t.Fatalf("ReadRef(): %v", err)
	}
	if read != hash {
		t.Fatalf("expected %s, got %s", hash, read)
	}

	_, err = ReadRef("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent ref")
	}
}

// ---------------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------------

func TestConcurrentReads(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	obj := NewBlobObject("concurrent test data")
	hash, err := WriteObject(obj)
	if err != nil {
		t.Fatalf("WriteObject(): %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			readObj, err := ReadObject(hash)
			if err != nil {
				t.Errorf("concurrent ReadObject(): %v", err)
			}
			if readObj.Data != "concurrent test data" {
				t.Errorf("data mismatch")
			}
		}()
	}
	wg.Wait()
}

func TestConcurrentWrites(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			obj := NewBlobObject("concurrent write")
			hash, err := WriteObject(obj)
			if err != nil {
				t.Errorf("concurrent WriteObject(): %v", err)
				return
			}
			if !ObjectExists(hash) {
				t.Errorf("object should exist after write")
			}
		}(i)
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// FormatDiff smoke test
// ---------------------------------------------------------------------------

func TestFormatDiffSmoke(t *testing.T) {
	makeHash := func(c byte) string {
		return strings.Repeat(string(c), 64)
	}
	result := &DiffResult{
		CommitA: makeHash('a'),
		CommitB: makeHash('b'),
		TreeDiffs: []DiffEntry{
			{Name: "prompt", Type: "blob", Status: "modified", OldHash: makeHash('1'), NewHash: makeHash('2'), TextDiff: "--- a\n+++ b\n-old\n+new\n"},
			{Name: "tool", Type: "blob", Status: "added", NewHash: makeHash('3')},
			{Name: "config", Type: "blob", Status: "removed", OldHash: makeHash('4')},
			{Name: "(metric) score", Status: "metric", MetricDiffs: []MetricDiff{{Key: "score", OldVal: 0.9, NewVal: 0.95, Delta: 0.05}}},
		},
	}

	output := FormatDiff(result)
	if output == "" {
		t.Fatal("expected non-empty diff output")
	}
	if !strings.Contains(output, "MODIFIED") {
		t.Fatal("expected 'MODIFIED' in output")
	}
	if !strings.Contains(output, "METRIC") {
		t.Fatal("expected 'METRIC' in output")
	}
}

// ---------------------------------------------------------------------------
// Inspection functions
// ---------------------------------------------------------------------------

func TestGetStatsEmptyRepo(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	stats, err := GetStats()
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if stats.TotalObjects != 0 {
		t.Fatalf("expected 0 total objects, got %d", stats.TotalObjects)
	}
	if stats.RefCount != 1 {
		t.Fatalf("expected 1 ref, got %d", stats.RefCount)
	}
	if stats.StorageBytes != 0 {
		t.Fatalf("expected 0 storage bytes, got %d", stats.StorageBytes)
	}
}

func TestGetStatsWithObjects(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b := NewBlobObject("test data")
	_, _ = WriteObject(b)

	stats, err := GetStats()
	if err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if stats.TotalObjects != 1 {
		t.Fatalf("expected 1 total object, got %d", stats.TotalObjects)
	}
	if stats.BlobCount != 1 {
		t.Fatalf("expected 1 blob, got %d", stats.BlobCount)
	}
	if stats.StorageBytes <= 0 {
		t.Fatalf("expected positive storage bytes, got %d", stats.StorageBytes)
	}
}

func TestVerifyHealthyRepo(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b := NewBlobObject("test")
	_, _ = WriteObject(b)

	result, err := Verify()
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if result.TotalObjects != 1 {
		t.Fatalf("expected 1 total object, got %d", result.TotalObjects)
	}
	if result.VerifiedObjects != 1 {
		t.Fatalf("expected 1 verified, got %d", result.VerifiedObjects)
	}
	if result.CorruptedObjects != 0 {
		t.Fatalf("expected 0 corrupted, got %d", result.CorruptedObjects)
	}
}

func TestListRefDetails(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	details, err := ListRefDetails()
	if err != nil {
		t.Fatalf("ListRefDetails() error: %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("expected 1 ref detail, got %d", len(details))
	}
	if details[0].Name != "main" {
		t.Fatalf("expected 'main', got %q", details[0].Name)
	}
	if details[0].Hash != "" {
		t.Fatalf("expected empty hash, got %q", details[0].Hash)
	}
}

func TestBuildGraphEmpty(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	nodes, err := BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}
	if nodes != nil {
		t.Fatal("expected nil for empty repo")
	}
}

func TestBuildGraphWithCommits(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b := NewBlobObject("data")
	bh, _ := WriteObject(b)
	th, _ := WriteObject(NewTreeObject([]TreeEntry{{Name: "file", Type: TypeBlob, Hash: bh}}))
	_, _ = Commit(th, nil, "author", "first", nil)

	nodes, err := BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if !nodes[0].IsHEAD {
		t.Fatal("expected HEAD to be set")
	}
	if len(nodes[0].Branches) != 1 || nodes[0].Branches[0] != "main" {
		t.Fatalf("expected branch 'main', got %v", nodes[0].Branches)
	}
}

func TestBuildGraphMultiBranch(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b := NewBlobObject("data")
	bh, _ := WriteObject(b)
	th, _ := WriteObject(NewTreeObject([]TreeEntry{{Name: "file", Type: TypeBlob, Hash: bh}}))
	c1, _ := Commit(th, nil, "author", "first", nil)

	_ = CreateBranch("feature", "")
	_ = Checkout("feature")
	_, _ = Commit(th, []string{c1.Hash}, "author", "feature commit", nil)

	_ = Checkout("main")

	nodes, err := BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	mainFound := false
	featureFound := false
	for _, n := range nodes {
		for _, b := range n.Branches {
			if b == "main" {
				mainFound = true
			}
			if b == "feature" {
				featureFound = true
			}
		}
	}
	if !mainFound {
		t.Fatal("expected 'main' branch to appear in graph")
	}
	if !featureFound {
		t.Fatal("expected 'feature' branch to appear in graph")
	}
}

func TestGetStatsUninitialized(t *testing.T) {
	_ = os.RemoveAll(PromptsheonDir)
	defer func() { _ = os.RemoveAll(PromptsheonDir) }()

	_, err := GetStats()
	if err != ErrRepoNotInitialized {
		t.Fatalf("expected ErrRepoNotInitialized, got %v", err)
	}
}

func TestVerifyUninitialized(t *testing.T) {
	_ = os.RemoveAll(PromptsheonDir)
	defer func() { _ = os.RemoveAll(PromptsheonDir) }()

	_, err := Verify()
	if err != ErrRepoNotInitialized {
		t.Fatalf("expected ErrRepoNotInitialized, got %v", err)
	}
}

func TestBuildGraphUninitialized(t *testing.T) {
	_ = os.RemoveAll(PromptsheonDir)
	defer func() { _ = os.RemoveAll(PromptsheonDir) }()

	_, err := BuildGraph()
	if err != ErrRepoNotInitialized {
		t.Fatalf("expected ErrRepoNotInitialized, got %v", err)
	}
}

func TestBuildGraphMergeCommit(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b := NewBlobObject("data")
	bh, _ := WriteObject(b)
	th, _ := WriteObject(NewTreeObject([]TreeEntry{{Name: "file", Type: TypeBlob, Hash: bh}}))

	c1, _ := Commit(th, nil, "author", "root", nil)
	_ = Checkout(c1.Hash)
	c2, _ := Commit(th, []string{c1.Hash}, "author", "feature", nil)
	_ = Checkout("main")
	c3, _ := Commit(th, []string{c1.Hash}, "author", "main second", nil)
	_, _ = Commit(th, []string{c3.Hash, c2.Hash}, "author", "merge", nil)

	nodes, err := BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}
	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}
}

func TestBuildGraphDetachedHead(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b := NewBlobObject("data")
	bh, _ := WriteObject(b)
	th, _ := WriteObject(NewTreeObject([]TreeEntry{{Name: "file", Type: TypeBlob, Hash: bh}}))

	c1, _ := Commit(th, nil, "author", "first", nil)
	_ = Checkout(c1.Hash)

	nodes, err := BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph() error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if !nodes[0].IsHEAD {
		t.Fatal("expected HEAD to be set for detached HEAD")
	}
	if len(nodes[0].Branches) != 1 || nodes[0].Branches[0] != "main" {
		t.Fatalf("expected main branch, got %v", nodes[0].Branches)
	}
}

func TestListRefDetailsAfterCommit(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b := NewBlobObject("data")
	bh, _ := WriteObject(b)
	th, _ := WriteObject(NewTreeObject([]TreeEntry{{Name: "file", Type: TypeBlob, Hash: bh}}))
	_, _ = Commit(th, nil, "author", "first", nil)

	details, err := ListRefDetails()
	if err != nil {
		t.Fatalf("ListRefDetails() error: %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("expected 1 ref detail, got %d", len(details))
	}
	if details[0].Name != "main" {
		t.Fatalf("expected 'main', got %q", details[0].Name)
	}
	if len(details[0].Hash) != 64 {
		t.Fatalf("expected 64-char hash, got %q", details[0].Hash)
	}
}

// ---------------------------------------------------------------------------
// computeTextDiff edge cases
// ---------------------------------------------------------------------------

func TestComputeTextDiffIdentical(t *testing.T) {
	result := computeTextDiff("hello\nworld", "hello\nworld", "file")
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if len(line) > 0 && (line[0] == '-' || line[0] == '+') {
			// Skip the header lines which start with --- or +++
			if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
				continue
			}
			t.Fatalf("identical texts should produce no +/- diff lines, got line: %q", line)
		}
	}
}

func TestComputeTextDiffEmpty(t *testing.T) {
	result := computeTextDiff("", "", "file")
	if result == "" {
		t.Fatal("expected non-empty header even for empty texts")
	}
	if !strings.Contains(result, "--- file (old)") {
		t.Fatalf("expected header, got:\n%s", result)
	}
}

func TestComputeTextDiffAddedLines(t *testing.T) {
	result := computeTextDiff("", "new line", "file")
	if !strings.Contains(result, "+new line") {
		t.Fatalf("expected added line, got:\n%s", result)
	}
}

func TestComputeTextDiffRemovedLines(t *testing.T) {
	result := computeTextDiff("old line", "", "file")
	if !strings.Contains(result, "-old line") {
		t.Fatalf("expected removed line, got:\n%s", result)
	}
}

// ---------------------------------------------------------------------------
// toFloat64 coverage
// ---------------------------------------------------------------------------

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		want   float64
		wantOk bool
	}{
		{"float64", float64(1.5), 1.5, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", int(3), 3, true},
		{"int64", int64(4), 4, true},
		{"int32", int32(5), 5, true},
		{"uint", uint(6), 6, true},
		{"uint64", uint64(7), 7, true},
		{"json.Number", json.Number("8.5"), 8.5, true},
		{"string", "not a number", 0, false},
		{"nil", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			if ok != tt.wantOk {
				t.Fatalf("toFloat64(%v) ok = %v, want %v", tt.input, ok, tt.wantOk)
			}
			if ok && got != tt.want {
				t.Fatalf("toFloat64(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HEADRefName edge cases
// ---------------------------------------------------------------------------

func TestHEADRefName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"symbolic", "ref: refs/heads/main", "main"},
		{"detached", "abc123", ""},
		{"bare ref prefix", "ref: refs/heads/", ""},
		{"malformed refs", "ref: refs/other/main", "refs/other/main"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HEADRefName(tt.input)
			if got != tt.want {
				t.Fatalf("HEADRefName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateBranchName max length
// ---------------------------------------------------------------------------

func TestValidateBranchNameMaxLength(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b := NewBlobObject("data")
	bh, _ := WriteObject(b)
	th, _ := WriteObject(NewTreeObject([]TreeEntry{{Name: "file", Type: TypeBlob, Hash: bh}}))
	_, _ = Commit(th, nil, "author", "init", nil)

	// Exactly at limit should succeed.
	validName := strings.Repeat("a", maxBranchLength)
	if err := CreateBranch(validName, ""); err != nil {
		t.Fatalf("branch name at max length should succeed: %v", err)
	}

	// Over the limit should fail.
	invalidName := strings.Repeat("a", maxBranchLength+1)
	if err := CreateBranch(invalidName, ""); err == nil {
		t.Fatal("branch name over max length should fail")
	}
}

// ---------------------------------------------------------------------------
// NewCommitObject nil vs empty telemetry
// ---------------------------------------------------------------------------

func TestNewCommitObjectNilTelemetry(t *testing.T) {
	c := NewCommitObject("tree", nil, "author", "msg", nil)
	if len(c.Telemetry) != 0 {
		t.Fatalf("expected 0 telemetry, got %d", len(c.Telemetry))
	}
	if len(c.Parents) != 0 {
		t.Fatalf("expected 0 parents, got %d", len(c.Parents))
	}
}

func TestNewCommitObjectEmptyTelemetry(t *testing.T) {
	c := NewCommitObject("tree", nil, "author", "msg", map[string]any{})
	if len(c.Telemetry) != 0 {
		t.Fatalf("expected 0 telemetry for empty map, got %d", len(c.Telemetry))
	}
}

func TestNewCommitObjectParentsSorted(t *testing.T) {
	c := NewCommitObject("tree", []string{"c", "a", "b"}, "author", "msg", nil)
	if len(c.Parents) != 3 {
		t.Fatalf("expected 3 parents, got %d", len(c.Parents))
	}
	if c.Parents[0] != "a" || c.Parents[1] != "b" || c.Parents[2] != "c" {
		t.Fatalf("parents not sorted: %v", c.Parents)
	}
}

// ---------------------------------------------------------------------------
// FormatDiff edge cases
// ---------------------------------------------------------------------------

func TestFormatDiffEmpty(t *testing.T) {
	makeHash := func(c byte) string {
		return strings.Repeat(string(c), 64)
	}
	result := &DiffResult{
		CommitA: makeHash('a'),
		CommitB: makeHash('b'),
	}
	output := FormatDiff(result)
	if !strings.Contains(output, "Intelligence Diff") {
		t.Fatalf("expected header in empty diff output, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// WriteObject atomicity
// ---------------------------------------------------------------------------

func TestWriteObjectConcurrent(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	var wg sync.WaitGroup
	hashes := make([]string, 10)
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			obj := NewBlobObject(fmt.Sprintf("concurrent content %d", i))
			hashes[i], errs[i] = WriteObject(obj)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: WriteObject: %v", i, err)
		}
		if len(hashes[i]) != 64 {
			t.Errorf("goroutine %d: expected 64-char hash, got %d", i, len(hashes[i]))
		}
	}
}

func TestWriteObjectDedup(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	obj := NewBlobObject("dedup test")
	h1, err := WriteObject(obj)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	h2, err := WriteObject(obj)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("dedup failed: %s vs %s", h1, h2)
	}
}

// ---------------------------------------------------------------------------
// DiffIntelligence type validation
// ---------------------------------------------------------------------------

func TestDiffIntelligenceWithBlobHash(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b := NewBlobObject("not a commit")
	hash, _ := WriteObject(b)

	_, err := DiffIntelligence(hash, hash)
	if err == nil {
		t.Fatal("expected error when passing a blob hash to DiffIntelligence")
	}
}

// ---------------------------------------------------------------------------
// Verify with corrupted object
// ---------------------------------------------------------------------------

func TestVerifyCorruptedObject(t *testing.T) {
	setupTestRepo(t)
	defer teardownTestRepo(t)

	b := NewBlobObject("verify me")
	hash, _ := WriteObject(b)

	// Corrupt the object file by truncating it.
	// Objects are stored read-only (0444), so we must chmod first.
	objPath := filepath.Join(PromptsheonDir, objectsDir, hash[:2], hash[2:])
	if err := os.Chmod(objPath, 0644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	data, err := os.ReadFile(objPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if e := os.WriteFile(objPath, data[:len(data)/2], 0644); e != nil {
		t.Fatalf("WriteFile: %v", e)
	}
	_ = os.Chmod(objPath, 0444)

	result, err := Verify()
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.CorruptedObjects == 0 {
		t.Fatal("expected at least 1 corrupted object")
	}
}
