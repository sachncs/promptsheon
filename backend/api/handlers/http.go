package handlers

import (
	"encoding/json"
	"net/http"
)

type Func func(http.ResponseWriter, *http.Request) error

func JSON(w http.ResponseWriter, status int, value any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(value)
}
