package api

import (
	"net/http"

	"github.com/sachn-cs/promptsheon/internal/promptsheon"
)

func (s *Server) handlePromptDiff(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if id == "" {
		return badRequest("id is required")
	}

	v1Hash := r.URL.Query().Get("v1")
	v2Hash := r.URL.Query().Get("v2")
	if v1Hash == "" || v2Hash == "" {
		return badRequest("v1 and v2 query parameters are required")
	}

	// Fetch both versions from CAS
	obj1, err := promptsheon.ReadObject(v1Hash)
	if err != nil {
		return badRequest("v1 object not found: " + v1Hash)
	}

	obj2, err := promptsheon.ReadObject(v2Hash)
	if err != nil {
		return badRequest("v2 object not found: " + v2Hash)
	}

	// Compare the objects
	diff := map[string]any{
		"v1_hash":       v1Hash,
		"v2_hash":       v2Hash,
		"v1_data":       obj1.Data,
		"v2_data":       obj2.Data,
		"v1_tree_hash":  obj1.TreeHash,
		"v2_tree_hash":  obj2.TreeHash,
		"v1_timestamp":  obj1.Timestamp,
		"v2_timestamp":  obj2.Timestamp,
		"v1_message":    obj1.Message,
		"v2_message":    obj2.Message,
		"same_content":  obj1.Data == obj2.Data && obj1.TreeHash == obj2.TreeHash,
	}

	writeJSON(w, http.StatusOK, diff)
	return nil
}
