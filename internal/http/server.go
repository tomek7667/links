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
	Links() []domain.Link
	CreateLink(link domain.Link, at int) (domain.Link, error)
	UpdateLink(id string, link domain.Link) (domain.Link, error)
	DeleteLink(id string) error
	DeleteLinkByURL(url string) error
	ReorderLinks(ids []string) error
	ReplaceHost(from, to string) (int, error)
	RenameGroup(from, to string) (int, error)
	ImportLinks(links []domain.Link, replace bool) (added, skipped int, err error)
	Settings() domain.Settings
	SaveSettings(s domain.Settings) (domain.Settings, error)
	Close()
}

type Server struct {
	port int
	dber Dber
	r    *chi.Mux

	resources *ResourceMonitor
	checker   *linkChecker
}

func New(port int, dber Dber) *Server {
	s := &Server{
		r:         chi.NewRouter(),
		port:      port,
		dber:      dber,
		resources: NewResourceMonitor(),
		checker:   newLinkChecker(),
	}
	s.r.Use(newRequestLogger("/api/resources", "/api/links/status"))
	s.r.Use(middleware.RequestID)
	s.r.Use(middleware.RealIP)
	s.r.Use(middleware.Recoverer)
	s.r.Use(middleware.GetHead)
	s.r.Use(middleware.Timeout(60 * time.Second))
	s.r.Use(middleware.Compress(5, "text/html", "text/css", "text/javascript", "application/javascript", "application/json", "image/svg+xml"))
	return s
}

func (s *Server) Serve() error {
	stopResources := make(chan struct{})
	s.resources.Start(stopResources)
	defer close(stopResources)
	defer s.dber.Close()

	s.addRoutes()

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

// effectiveHostIP is the IP substituted for {ip}: the settings override, or
// the auto-detected address.
func (s *Server) effectiveHostIP() string {
	if o := s.dber.Settings().HostIP; o != "" {
		return o
	}
	return s.resources.DetectedHostIP()
}
