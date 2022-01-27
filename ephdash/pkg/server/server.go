package server

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/tilt-dev/ephemerator/ephconfig"
	"github.com/tilt-dev/ephemerator/ephdash/web/static"
	webtemplate "github.com/tilt-dev/ephemerator/ephdash/web/template"
)

// HTTP Handler for application-level logic.
type Server struct {
	*mux.Router

	allowlist *ephconfig.Allowlist
	tmpl      *template.Template
}

func NewServer(allowlist *ephconfig.Allowlist) (*Server, error) {
	s := &Server{allowlist: allowlist}

	r := mux.NewRouter()
	staticContent := http.FileServer(http.FS(static.Content))
	tmpl, err := template.ParseFS(webtemplate.Content, "*.tmpl")
	if err != nil {
		return nil, err
	}

	r.Handle("/favicon.ico", staticContent)
	r.HandleFunc("/index.html", s.index)
	r.HandleFunc("/", s.index)
	s.Router = r
	s.tmpl = tmpl
	return s, nil
}

func (s *Server) index(res http.ResponseWriter, r *http.Request) {
	err := s.tmpl.ExecuteTemplate(res, "index.tmpl", map[string]interface{}{
		"allowlist": s.allowlist,
	})
	if err != nil {
		http.Error(res, fmt.Sprintf("Rendering HTML: %v", err), http.StatusInternalServerError)
	}
}
