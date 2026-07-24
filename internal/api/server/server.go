package server

import (
	"log/slog"

	"github.com/sachncs/promptsheon/internal/api"
	"github.com/sachncs/promptsheon/internal/store"
)

type Server = api.Server
type Option = api.Option

func New(db *store.Repositories, logger *slog.Logger, opts ...Option) *Server {
	return api.NewServer(db, logger, opts...)
}
