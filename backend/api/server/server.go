package server

import (
	"log/slog"

	"github.com/sachncs/promptsheon/backend/api"
	"github.com/sachncs/promptsheon/backend/store"
)

type Server = api.Server
type Option = api.Option

func New(db *store.Repositories, logger *slog.Logger, opts ...Option) *Server {
	return api.NewServer(db, logger, opts...)
}
