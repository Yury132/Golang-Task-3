package http

import (
	"net/http"

	"github.com/Yury132/Golang-Task-3/internal/transport/http/handlers"
)

type Server struct {
	*http.Server
}

func New(addr string) *Server {
	return &Server{
		&http.Server{
			Addr:         addr,
			WriteTimeout: http.DefaultClient.Timeout,
			ReadTimeout:  http.DefaultClient.Timeout,
		},
	}
}

func (s *Server) WithHandler(handler *handlers.Handler) *Server {
	s.Handler = InitRoutes(handler)
	return s
}

func (s *Server) Run() error {
	if err := s.ListenAndServe(); err != nil {
		return err
	}

	return nil
}
