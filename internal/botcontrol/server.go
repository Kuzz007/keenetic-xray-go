package botcontrol

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// RouterAuth resolves a router ID to its expected bearer token, and
// reports whether that router is known at all. Separated from Server so
// tests can supply a trivial in-memory implementation.
type RouterAuth interface {
	TokenFor(routerID string) (token string, ok bool)
}

// StaticRouterAuth is a fixed router-ID -> token map, typically loaded
// once at startup from the control server's own config file. Registering
// a new router means adding it there and restarting.
type StaticRouterAuth map[string]string

// TokenFor implements RouterAuth.
func (m StaticRouterAuth) TokenFor(routerID string) (string, bool) {
	token, ok := m[routerID]
	return token, ok
}

// ServerConfig configures NewServer.
type ServerConfig struct {
	Store       *Store
	Auth        RouterAuth
	Fingerprint string      // hex SHA256 of the serving certificate's leaf, served unauthenticated at /fingerprint
	Logger      *log.Logger // nil -> log.Default()
}

// Server is the control-server's HTTP API: unauthenticated /fingerprint
// for first-trust bootstrapping (an operator reads it once, out of band,
// to configure a new agent), and Bearer+X-Router-Id-authenticated
// /agent/poll and /agent/result for the router agents.
type Server struct {
	cfg ServerConfig
	mux *http.ServeMux
}

// NewServer builds a Server ready to be wrapped in an *http.Server (see
// ListenAndServeTLS).
func NewServer(cfg ServerConfig) *Server {
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.mux.HandleFunc("/fingerprint", s.handleFingerprint)
	s.mux.HandleFunc("/agent/poll", s.authenticated(s.handlePoll))
	s.mux.HandleFunc("/agent/result", s.authenticated(s.handleResult))
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) handleFingerprint(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, s.cfg.Fingerprint)
}

// authenticated wraps next with Bearer-token authentication. The router
// ID comes from RouterIDHeader -- needed to know which token to check
// against before (and regardless of) parsing any body, since /agent/poll
// has no body at all -- and the token from the Authorization header; the
// two are compared constant-time so a timing side-channel can't be used
// to brute-force a valid token.
func (s *Server) authenticated(next func(w http.ResponseWriter, r *http.Request, routerID string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		routerID := r.Header.Get(RouterIDHeader)
		if routerID == "" {
			http.Error(w, "missing "+RouterIDHeader, http.StatusUnauthorized)
			return
		}
		want, ok := s.cfg.Auth.TokenFor(routerID)
		if !ok {
			http.Error(w, "unknown router", http.StatusUnauthorized)
			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		next(w, r, routerID)
	}
}

func (s *Server) handlePoll(w http.ResponseWriter, _ *http.Request, routerID string) {
	cmd, err := s.cfg.Store.Dequeue(routerID)
	if err != nil {
		s.cfg.Logger.Printf("dequeue for %s: %v", routerID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(PollResponse{Command: cmd})
}

func (s *Server) handleResult(w http.ResponseWriter, r *http.Request, routerID string) {
	var result Result
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := s.cfg.Store.RecordResult(routerID, result); err != nil {
		s.cfg.Logger.Printf("record result for %s: %v", routerID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ListenAndServeTLS runs handler as an HTTPS server on addr using cert
// until ctx is cancelled, at which point it shuts down gracefully (5s
// grace period) and returns ctx.Err().
func ListenAndServeTLS(ctx context.Context, addr string, cert tls.Certificate, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}},
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1) // buffered: the goroutine must never block on a send nobody reads
	go func() { errCh <- srv.ListenAndServeTLS("", "") }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return ctx.Err()
	}
}
