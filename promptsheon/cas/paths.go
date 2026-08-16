package cas

import (
	"os"
	"path/filepath"

	"github.com/sachncs/promptsheon/errf"
)

// branchRefPath returns the relative path of a branch ref inside PromptsheonDir.
func branchRefPath(name string) string {
	return filepath.Join(headsDir, name)
}

// openRepoRoot returns an os.Root scoped to the repository directory.
// Callers must Close() the returned root.
func openRepoRoot() (*os.Root, error) {
	root, err := os.OpenRoot(PromptsheonDir)
	if err != nil {
		return nil, errf.Errorf("open repo: %w", err)
	}
	return root, nil
}
