package cas

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// VerifyResult is the structured output of Verify. The counters
// are non-negative; Errors is a flat list of human-readable
// failure descriptions suitable for printing to a terminal.
type VerifyResult struct {
	TotalObjects     int
	VerifiedObjects  int
	CorruptedObjects int
	BrokenRefs       int
	OrphanedObjects  int
	Errors           []string
}

// Verify walks every object in the CAS, recomputes its hash, and
// reports any discrepancies. It also walks the refs and the
// reachable set so it can flag broken refs (pointing at missing
// objects) and orphan objects (not reachable from any ref).
//
// Verify never returns an error: the result struct is the source
// of truth. A non-empty Errors slice means the repository needs
// attention.
func Verify() (*VerifyResult, error) {
	if !IsInitialized() {
		return nil, ErrRepoNotInitialized
	}
	result := &VerifyResult{}

	allObjects, err := collectAllObjects(result)
	if err != nil {
		return nil, err
	}

	verifyObjects(result, allObjects)

	refs, err := checkRefs(result, allObjects)
	if err != nil {
		return nil, err
	}

	findOrphans(result, allObjects, refs)
	return result, nil
}

func collectAllObjects(result *VerifyResult) (map[string]string, error) {
	allObjects := make(map[string]string)
	objectsDirPath := filepath.Join(PromptsheonDir, objectsDir)
	shards, err := os.ReadDir(objectsDirPath)
	if err != nil {
		return nil, fmt.Errorf("read objects: %w", err)
	}
	for _, shard := range shards {
		if !shard.IsDir() || len(shard.Name()) != 2 {
			continue
		}
		entries, e := os.ReadDir(filepath.Join(objectsDirPath, shard.Name()))
		if e != nil {
			return nil, fmt.Errorf("read shard %s: %w", shard.Name(), e)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			hash := shard.Name() + e.Name()
			allObjects[hash] = filepath.Join(objectsDirPath, shard.Name(), e.Name())
			result.TotalObjects++
		}
	}
	return allObjects, nil
}

func verifyObjects(result *VerifyResult, allObjects map[string]string) {
	for hash, path := range allObjects {
		obj, e := ReadObject(hash)
		if e != nil {
			result.CorruptedObjects++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", shortHash(hash), e))
			continue
		}
		recomputed, err := ObjectHash(obj)
		if err != nil {
			result.CorruptedObjects++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: recompute: %v", shortHash(hash), err))
			continue
		}
		if recomputed != hash {
			result.CorruptedObjects++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: hash mismatch (computed %s)", shortHash(hash), shortHash(recomputed)))
			continue
		}
		result.VerifiedObjects++
		_ = path
	}
}

func checkRefs(result *VerifyResult, allObjects map[string]string) ([]string, error) {
	refs, err := ListRefs()
	if err != nil {
		return nil, err
	}
	for _, r := range refs {
		hash, err := ReadRef(r)
		if err != nil {
			if errors.Is(err, ErrRefNotFound) {
				continue
			}
			result.BrokenRefs++
			result.Errors = append(result.Errors, fmt.Sprintf("ref %s: %v", r, err))
			continue
		}
		if hash == "" {
			continue
		}
		if _, ok := allObjects[hash]; !ok {
			result.BrokenRefs++
			result.Errors = append(result.Errors, fmt.Sprintf("ref %s -> missing object %s", r, shortHash(hash)))
		}
	}
	return refs, nil
}

func findOrphans(result *VerifyResult, allObjects map[string]string, refs []string) {
	reachable := make(map[string]struct{})
	for _, r := range refs {
		hash, err := ReadRef(r)
		if err != nil || hash == "" {
			continue
		}
		walkReachable(hash, reachable)
	}
	for h := range allObjects {
		if _, ok := reachable[h]; !ok {
			result.OrphanedObjects++
		}
	}
}

// walkReachable performs a BFS from hash following the Parents
// links on commit objects. Non-commit objects are added but not
// recursed into. Cycles are tolerated because we track visited
// hashes.
func walkReachable(hash string, out map[string]struct{}) {
	queue := []string{hash}
	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]
		if _, seen := out[h]; seen {
			continue
		}
		out[h] = struct{}{}
		obj, err := ReadObject(h)
		if err != nil {
			continue
		}
		if obj.IsCommit() {
			queue = append(queue, obj.Parents...)
		}
	}
}
