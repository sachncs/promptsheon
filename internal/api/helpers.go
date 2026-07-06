package api

import (
	"fmt"
	"time"
)

func generateID() string {
	return fmt.Sprintf("api-%d", time.Now().UnixNano())
}
