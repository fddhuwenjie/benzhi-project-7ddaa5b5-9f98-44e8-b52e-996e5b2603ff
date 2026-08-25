package httpui

import (
	"embed"
	"io/fs"
	"net/http"

	"benzhi-project-7ddaa5b5-9f98-44e8-b52e-996e5b2603ff/internal/application"
)

//go:embed web/*
var webFiles embed.FS

type Server struct {
	service *application.Service
	mux     *http.ServeMux
}

func New(service *application.Service) *Server {
	s := &Server{service: service, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(s.mux)
}

func (s *Server) routes() {
	assets, _ := fs.Sub(webFiles, "web")
	s.mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	s.mux.HandleFunc("GET /", s.Workbench)
	s.mux.HandleFunc("GET /healthz", s.Health)
	s.mux.HandleFunc("GET /api/rules", s.GetRules)
	s.mux.HandleFunc("GET /api/cases", s.ListCases)
	s.mux.HandleFunc("POST /api/cases", s.CreateCase)
	s.mux.HandleFunc("GET /api/cases/{id}", s.GetCase)
	s.mux.HandleFunc("PATCH /api/cases/{id}", s.UpdateDraft)
	s.mux.HandleFunc("POST /api/cases/{id}/validate", s.ValidateCase)
	s.mux.HandleFunc("POST /api/cases/{id}/review", s.StartReview)
	s.mux.HandleFunc("POST /api/cases/{id}/opinions", s.SubmitOpinion)
	s.mux.HandleFunc("POST /api/cases/{id}/revisions", s.SubmitRevision)
	s.mux.HandleFunc("POST /api/cases/{id}/approval", s.ApproveCase)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; object-src 'none'; base-uri 'none'")
		next.ServeHTTP(w, r)
	})
}
