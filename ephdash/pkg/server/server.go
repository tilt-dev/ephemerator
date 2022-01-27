package server

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/tilt-dev/ephemerator/ephconfig"
	"github.com/tilt-dev/ephemerator/ephdash/pkg/env"
	"github.com/tilt-dev/ephemerator/ephdash/web/static"
	webtemplate "github.com/tilt-dev/ephemerator/ephdash/web/template"
)

// HTTP Handler for application-level logic.
type Server struct {
	*mux.Router

	envClient *env.Client
	allowlist *ephconfig.Allowlist
	tmpl      *template.Template
}

func NewServer(envClient *env.Client, allowlist *ephconfig.Allowlist) (*Server, error) {
	s := &Server{envClient: envClient, allowlist: allowlist}

	r := mux.NewRouter()
	staticContent := http.FileServer(http.FS(static.Content))
	tmpl, err := template.ParseFS(webtemplate.Content, "*.tmpl")
	if err != nil {
		return nil, err
	}

	r.Handle("/favicon.ico", staticContent).Methods("GET")
	r.HandleFunc("/index.html", s.index).Methods("GET")
	r.HandleFunc("/", s.index).Methods("GET")

	s.Router = r
	s.tmpl = tmpl
	return s, nil
}

func (s *Server) index(res http.ResponseWriter, r *http.Request) {
	user := "nicks" // TODO(nick): Get an actual authenticated user.
	env, envError := s.envClient.GetEnv(r.Context(), user)
	err := s.tmpl.ExecuteTemplate(res, "index.tmpl", map[string]interface{}{
		"allowlist": s.allowlist,
		"env":       env,
		"envError":  envError,
	})
	if err != nil {
		http.Error(res, fmt.Sprintf("Rendering HTML: %v", err), http.StatusInternalServerError)
	}
}
