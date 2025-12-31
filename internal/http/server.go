package http

import (
	"context"
	"errors"
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

	resources *ResourceMonitor
}

func New(port int, dber Dber) *Server {
	s := &Server{
		r:         chi.NewRouter(),
		port:      port,
		dber:      dber,
		resources: NewResourceMonitor(),
	}
	s.r.Use(newRequestLogger("/api/resources"))
	s.r.Use(middleware.RequestID)
	s.r.Use(middleware.RealIP)
	s.r.Use(middleware.Recoverer)
	s.r.Use(middleware.Timeout(60 * time.Second))
	return s
}

func (s *Server) Serve() error {
	stopResources := make(chan struct{})
	s.resources.Start(stopResources)
	defer close(stopResources)
	defer s.dber.Close()

	s.AddIndexRoute()

	addr := fmt.Sprintf(":%d", s.port)
	srv := &http.Server{
		Addr:    addr,
		Handler: s.r,
	}

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("listening on '%s'\n", addr)
		err := srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(c)

	select {
	case sig := <-c:
		fmt.Printf("received signal '%s', shutting down\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		return <-errCh
	case err := <-errCh:
		if err == nil {
			return nil
		}
		if errors.Is(err, syscall.EACCES) && s.port < 1024 {
			return fmt.Errorf("failed to listen on %s: %w (use --port 8080 or run with CAP_NET_BIND_SERVICE)", addr, err)
		}
		return err
	}
}
