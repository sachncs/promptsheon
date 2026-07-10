package cas

import (
	"fmt"
	"os"
	"path/filepath"
)

// RepoStats summarises a repository. All counters are non-negative
// and the StorageBytes is the sum of the compressed on-disk
// sizes of every object in the CAS.
type RepoStats struct {
	TotalObjects int
	BlobCount    int
	TreeCount    int
	CommitCount  int
	StorageBytes int64
	RefCount     int
}

// GetStats walks the CAS and the refs directory to compute a
// repository snapshot. The function is O(n) in the number of
// objects; for the sizes promptsheon typically holds (a few
// thousand objects) this is fast enough to call on every
// dashboard refresh.
func GetStats() (*RepoStats, error) {
	if !IsInitialized() {
		return nil, ErrRepoNotInitialized
	}
	stats := &RepoStats{}
	objectsDir := filepath.Join(PromptsheonDir, objectsDir)
	shards, err := os.ReadDir(objectsDir)
	if err != nil {
		return nil, fmt.Errorf("read objects: %w", err)
	}
	for _, shard := range shards {
		if !shard.IsDir() || len(shard.Name()) != 2 {
			continue
		}
		entries, e := os.ReadDir(filepath.Join(objectsDir, shard.Name()))
		if e != nil {
			return nil, fmt.Errorf("read shard %s: %w", shard.Name(), e)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr != nil {
				continue
			}
			stats.TotalObjects++
			stats.StorageBytes += info.Size()

			// Peek at the object to count by type. We do this in
			// the same pass as the file walk to avoid a second
			// directory traversal.
			hash := shard.Name() + entry.Name()
			obj, objErr := ReadObject(hash)
			if objErr != nil {
				// Corrupt or unreadable object: still count it
				// in TotalObjects and StorageBytes but skip the
				// type breakdown. Verify() will surface the
				// corruption detail.
				continue
			}
			switch obj.Type() {
			case TypeBlob:
				stats.BlobCount++
			case TypeTree:
				stats.TreeCount++
			case TypeCommit:
				stats.CommitCount++
			}
		}
	}

	refs, err := ListRefs()
	if err != nil {
		return nil, err
	}
	stats.RefCount = len(refs)
	return stats, nil
}
