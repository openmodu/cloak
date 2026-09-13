// Package http 是 HTTP 适配层：解析请求、调用用例、组装响应，不含业务判断。
package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/openmodu/cloak/internal/usecase"
	"github.com/openmodu/cloak/internal/usecase/masker"
	"github.com/openmodu/cloak/internal/usecase/proxy"
	"github.com/openmodu/cloak/internal/usecase/restorer"
)

// Server 持有用例对象与一份可选的访问口令。
type Server struct {
	masker       *masker.Masker
	restorer     *restorer.Restorer
	proxy        *proxy.Proxy
	conf         usecase.ConfigStore
	apiKey       string
	temperature  float32
	proxyFactory func(string) (*proxy.Proxy, error)
	log          *slog.Logger

	srv *http.Server
}

type Option func(*Server)

func WithTemperature(v float32) Option { return func(s *Server) { s.temperature = v } }
func WithProxyFactory(f func(string) (*proxy.Proxy, error)) Option {
	return func(s *Server) { s.proxyFactory = f }
}

// WithAPIKey 开启 Authorization 校验。留空则不校验。
func WithAPIKey(key string) Option {
	return func(s *Server) { s.apiKey = key }
}

// WithProxy 接入 /api/call 需要的完整链路用例。未接入时该接口返回 503。
func WithProxy(p *proxy.Proxy) Option {
	return func(s *Server) { s.proxy = p }
}

func WithLogger(l *slog.Logger) Option {
	return func(s *Server) { s.log = l }
}

func New(m *masker.Masker, r *restorer.Restorer, conf usecase.ConfigStore, opts ...Option) *Server {
	s := &Server{masker: m, restorer: r, conf: conf, log: slog.Default()}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Handler 装配路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	registerWeb(mux)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/config", s.auth(s.handleConfig))
	mux.HandleFunc("POST /api/call", s.auth(s.handleCall))
	mux.HandleFunc("POST /api/mask_text", s.auth(s.handleMaskText))
	mux.HandleFunc("POST /api/restore_text", s.auth(s.handleRestoreText))
	mux.HandleFunc("POST /api/mask_text_batch", s.auth(s.handleMaskTextBatch))
	mux.HandleFunc("POST /api/restore_text_batch", s.auth(s.handleRestoreTextBatch))
	return mux
}

// ListenAndServe 启动服务，阻塞到 ctx 取消。
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	s.srv = &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(shutCtx)
	}()
	s.log.Info("cloakd 已启动", "addr", ln.Addr().String())
	if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// auth 校验 Authorization 头，支持裸 key 与 "Bearer <key>" 两种写法。
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.apiKey == "" {
			next(w, r)
			return
		}
		got := strings.TrimSpace(r.Header.Get("Authorization"))
		if len(got) >= 7 && strings.EqualFold(got[:7], "Bearer ") {
			got = got[7:]
		}
		if strings.TrimSpace(got) != s.apiKey {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeOutput(w http.ResponseWriter, output any) {
	writeJSON(w, http.StatusOK, envelope{Output: output})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, envelope{Error: &apiError{Message: msg}})
}

// decode 解析请求体，失败时直接回 400。
func decode[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20))
	if err := dec.Decode(&v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		var zero T
		return zero, false
	}
	return v, true
}
