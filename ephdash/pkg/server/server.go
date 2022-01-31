package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/tilt-dev/ephemerator/ephconfig"
	"github.com/tilt-dev/ephemerator/ephdash/pkg/env"
	"github.com/tilt-dev/ephemerator/ephdash/web/static"
	webtemplate "github.com/tilt-dev/ephemerator/ephdash/web/template"
)

// HTTP Handler for application-level logic.
type Server struct {
	*mux.Router

	envClient    *env.Client
	allowlist    *ephconfig.Allowlist
	gatewayHost  string
	tmpl         *template.Template
	authSettings AuthSettings
}

func NewServer(envClient *env.Client, allowlist *ephconfig.Allowlist, gatewayHost string, authSettings AuthSettings) (*Server, error) {
	s := &Server{envClient: envClient, allowlist: allowlist, gatewayHost: gatewayHost, authSettings: authSettings}

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
	user, err := s.username(r)
	if err != nil {
		http.Error(res, fmt.Sprintf("Reading username: %v", err), http.StatusInternalServerError)
		return
	}

	env, envError := s.envClient.GetEnv(r.Context(), user)
	err = s.tmpl.ExecuteTemplate(res, "index.tmpl", map[string]interface{}{
		"allowlist":   s.allowlist,
		"env":         env,
		"envError":    envError,
		"gatewayHost": s.gatewayHost,
		"user":        user,
	})
	if err != nil {
		http.Error(res, fmt.Sprintf("Rendering HTML: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) username(r *http.Request) (string, error) {
	if s.authSettings.FakeUser != "" {
		return s.authSettings.FakeUser, nil
	}

	url, err := url.Parse(s.authSettings.Proxy)
	if err != nil {
		return "", fmt.Errorf("fetching userinfo: %v", err)
	}

	url.Path = "/oauth2/userinfo"

	userInfoReq, err := http.NewRequest(
		"GET", url.String(), nil)
	if err != nil {
		return "", fmt.Errorf("fetching userinfo: %v", err)
	}

	// Forward all the cookies
	for _, c := range r.Cookies() {
		userInfoReq.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(userInfoReq)
	if err != nil {
		return "", fmt.Errorf("fetching userinfo: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching userinfo: bad status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	authResponse := AuthResponse{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&authResponse)
	if err != nil {
		return "", fmt.Errorf("parsing userinfo: %v", err)
	}
	if authResponse.User == "" {
		return "", fmt.Errorf("userinfo empty")
	}

	return authResponse.User, nil
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

	user, err := s.username(r)
	if err != nil {
		http.Error(res, fmt.Sprintf("Reading username: %v", err), http.StatusInternalServerError)
		return
	}

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

	http.Redirect(res, r, "/", http.StatusSeeOther)
}

// Deletes an environment. One environment per user.
func (s *Server) deleteEnv(res http.ResponseWriter, r *http.Request) {
	user, err := s.username(r)
	if err != nil {
		http.Error(res, fmt.Sprintf("Reading username: %v", err), http.StatusInternalServerError)
		return
	}

	err = s.envClient.DeleteEnv(r.Context(), user)
	if err != nil {
		http.Error(res, fmt.Sprintf("Deleting env: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, r, "/", http.StatusSeeOther)
}
