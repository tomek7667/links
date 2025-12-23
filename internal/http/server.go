package http

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tomek7667/links/internal/domain"
)

type Dber interface {
	SaveLink(link domain.Link)
	GetLinks() []domain.Link
	DeleteLink(url string)
	Close()
}

type Server struct {
	port int
	dber Dber
	r    *chi.Mux
}

func New(port int, dber Dber) *Server {
	s := &Server{
		r:    chi.NewRouter(),
		port: port,
		dber: dber,
	}
	s.r.Use(middleware.Logger)
	s.r.Use(middleware.RequestID)
	s.r.Use(middleware.RealIP)
	s.r.Use(middleware.Recoverer)
	s.r.Use(middleware.Timeout(60 * time.Second))
	return s
}

func (s *Server) Serve() {
	go func() {
		addr := fmt.Sprintf(":%d", s.port)
		fmt.Printf("listening on '%s'\n", addr)
		http.ListenAndServe(addr, s.r)
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	s.dber.Close()
}
