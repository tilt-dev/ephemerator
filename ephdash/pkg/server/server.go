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

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", staticContent))
	r.Handle("/favicon.ico", staticContent).Methods("GET")
	r.HandleFunc("/index.html", s.index).Methods("GET")
	r.HandleFunc("/create", s.create).Methods("POST")
	r.HandleFunc("/delete", s.deleteEnv).Methods("POST")
	r.HandleFunc("/", s.index).Methods("GET", "POST")

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

// Creates an environment.
//
// If there are fields missing, regenerates the creation
// form with options for the missing fields.
func (s *Server) create(res http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(res, fmt.Sprintf("Parsing form data: %v", err), http.StatusInternalServerError)
		return
	}

	user := "nicks" // TODO(nick): Get an actual authenticated user.
	spec := env.EnvSpec{
		Repo:   r.FormValue("repo"),
		Branch: r.FormValue("branch"),
		Path:   r.FormValue("path"),
	}

	if spec.Repo == "" || spec.Branch == "" || spec.Path == "" {
		http.Error(res, fmt.Sprintf("Missing form data: %v", spec), http.StatusBadRequest)
		return
	}

	err = ephconfig.IsAllowed(s.allowlist, spec.Repo)
	if err != nil {
		http.Error(res, fmt.Sprintf("May not create env for repo %q: %v", spec.Repo, err), http.StatusForbidden)
		return
	}

	err = s.envClient.SetEnvSpec(r.Context(), user, spec)
	if err != nil {
		http.Error(res, fmt.Sprintf("Creating env: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, r, "/", http.StatusTemporaryRedirect)
}

// Deletes an environment. One environment per user.
func (s *Server) deleteEnv(res http.ResponseWriter, r *http.Request) {
	user := "nicks" // TODO(nick): Get an actual authenticated user.
	err := s.envClient.DeleteEnv(r.Context(), user)
	if err != nil {
		http.Error(res, fmt.Sprintf("Deleting env: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, r, "/", http.StatusTemporaryRedirect)
}
