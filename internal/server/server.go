package server

import (
	"log"
	"net/http"
	"strings"

	"codex-service/internal/proxy"
)

type Server struct {
	handler    *proxy.Handler
	listenAddr string
	localAuth  string
}

func New(handler *proxy.Handler, listenAddr, localAuth string) *Server {
	return &Server{
		handler:    handler,
		listenAddr: listenAddr,
		localAuth:  localAuth,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("GET /v1/models", s.authMiddleware(s.handler.HandleModels))
	mux.HandleFunc("POST /v1/responses", s.authMiddleware(s.handler.HandleResponses))
	mux.HandleFunc("POST /v1/chat/completions", s.authMiddleware(s.handler.HandleChatCompletions))

	log.Printf("codex-service 啟動於 %s", s.listenAddr)
	log.Printf("端點:")
	log.Printf("  POST /v1/chat/completions")
	log.Printf("  POST /v1/responses")
	log.Printf("  GET  /v1/models")
	log.Printf("  GET  /health")

	return http.ListenAndServe(s.listenAddr, mux)
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.localAuth != "" {
			auth := r.Header.Get("Authorization")
			expected := "Bearer " + s.localAuth
			if !strings.EqualFold(auth, expected) {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}
