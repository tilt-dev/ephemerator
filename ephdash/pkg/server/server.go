package server

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v42/github"
	"github.com/gorilla/mux"
	"github.com/tilt-dev/ephemerator/ephconfig"
	"github.com/tilt-dev/ephemerator/ephdash/pkg/env"
	"github.com/tilt-dev/ephemerator/ephdash/web/static"
	"golang.org/x/oauth2"

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
	s := &Server{
		envClient:    envClient,
		allowlist:    allowlist,
		gatewayHost:  gatewayHost,
		authSettings: authSettings,
	}

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
		if r.Host != s.gatewayHost {
			res.WriteHeader(http.StatusNotFound)
			_ = s.tmpl.ExecuteTemplate(res, "not-found.tmpl", map[string]interface{}{
				"host":        r.Host,
				"gatewayHost": s.gatewayHost,
			})
			return
		}

		http.Error(res, fmt.Sprintf("Reading username: %v", err), http.StatusInternalServerError)
		return
	}

	env, envError := s.envClient.GetEnv(r.Context(), user)
	repoOptions, selectedRepo := s.repoOptions(r)
	githubClient := s.githubClient(r)
	branchOptions, selectedBranch := s.branchOptions(r, githubClient, selectedRepo)
	pathOptions := s.pathOptions(r, githubClient, selectedRepo, selectedBranch)

	err = s.tmpl.ExecuteTemplate(res, "index.tmpl", map[string]interface{}{
		"env":           env,
		"envError":      envError,
		"gatewayHost":   s.gatewayHost,
		"user":          user,
		"repoOptions":   repoOptions,
		"branchOptions": branchOptions,
		"pathOptions":   pathOptions,
	})
	if err != nil {
		http.Error(res, fmt.Sprintf("Rendering HTML: %v", err), http.StatusInternalServerError)
	}
}

func (s *Server) username(r *http.Request) (string, error) {
	if s.authSettings.FakeUser != "" {
		return s.authSettings.FakeUser, nil
	}

	user := r.Header.Get("X-Auth-Request-User")
	if user == "" {
		return "", fmt.Errorf("userinfo empty")
	}
	return user, nil
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

	spec := ephconfig.EnvSpec{
		Repo:   r.FormValue("repo"),
		Branch: r.FormValue("branch"),
		Path:   r.FormValue("path"),
	}

	if spec.Repo == "" || spec.Branch == "" || spec.Path == "" {
		http.Error(res, fmt.Sprintf("Missing form data: %v", spec), http.StatusBadRequest)
		return
	}

	err = ephconfig.IsAllowed(s.allowlist, spec)
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

type FormOption struct {
	Name     string
	Value    string
	Selected bool
}

// Generate all the valid options for the repo form.
// Returns the selected repo URL.
func (s *Server) repoOptions(r *http.Request) ([]FormOption, string) {
	result := []FormOption{}
	selected := ""
	qRepo := r.URL.Query().Get("repo")
	for _, repo := range s.allowlist.RepoNames {
		v := fmt.Sprintf("%s/%s", s.allowlist.RepoBase, repo)
		n := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(v, "http://"), "https://"), "github.com/")
		s := qRepo == v
		o := FormOption{
			Value:    v,
			Name:     n,
			Selected: s,
		}
		result = append(result, o)

		if s {
			selected = v
		}
	}
	if selected == "" && len(result) > 0 {
		result[0].Selected = true
		selected = result[0].Value
	}
	return result, selected
}

// Create a github go client.
// These use the users' auth token for rate-limiting, so
// need to be created per-request.
func (s *Server) githubClient(r *http.Request) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: r.Header.Get("X-Auth-Request-Access-Token")},
	)
	tc := oauth2.NewClient(r.Context(), ts)
	return github.NewClient(tc)
}

// Splits a github URL into an (owner, repoName) pair.
func (s *Server) toGithubOwnerAndRepo(repoURL string) (string, string) {
	if !strings.HasPrefix(repoURL, "https://github.com/") {
		return "", ""
	}

	path := strings.TrimPrefix(repoURL, "https://github.com/")
	split := strings.Split(path, "/")
	if len(split) < 2 {
		return "", ""
	}
	return split[0], split[1]
}

var defaultBranchName = "master"

// Generate a list of valid branches for the given repo.
// Returns the SHA hash of the selected branch.
func (s *Server) branchOptions(r *http.Request, client *github.Client, repoURL string) ([]FormOption, string) {
	result := []FormOption{}
	selected := ""

	branchList := []*github.Branch{&github.Branch{Name: &defaultBranchName}}
	owner, repoName := s.toGithubOwnerAndRepo(repoURL)
	if repoName != "" {
		branches, _, err := client.Repositories.ListBranches(r.Context(), owner, repoName, nil)
		if err != nil {
			log.Printf("error: fetching branches %s/%s: %v", owner, repoName, err)
		} else {
			branchList = branches
		}
	}

	qBranch := r.URL.Query().Get("branch")
	for _, b := range branchList {
		if b.Name == nil {
			continue
		}
		name := *b.Name
		s := qBranch == name
		o := FormOption{
			Value:    name,
			Name:     name,
			Selected: s,
		}
		result = append(result, o)

		if s && b.Commit != nil && b.Commit.SHA != nil {
			selected = *(b.Commit.SHA)
		}
	}

	if selected == "" && len(branchList) > 0 {
		result[0].Selected = true
		b := branchList[0]
		if b.Name != nil && *b.Name == result[0].Value && b.Commit != nil && b.Commit.SHA != nil {
			selected = *(b.Commit.SHA)
		}
	}

	return result, selected
}

// Generate a list of valid paths with Tiltfiles for the given repo/branch.
func (s *Server) pathOptions(r *http.Request, client *github.Client, repoURL, sha string) []FormOption {
	owner, repoName := s.toGithubOwnerAndRepo(repoURL)
	if repoName == "" {
		return nil
	}

	if sha == "" {
		return nil
	}

	tree, _, err := client.Git.GetTree(r.Context(), owner, repoName, sha, true /* recursive */)
	if err != nil {
		log.Printf("error: fetching tree %s/%s: %v", owner, repoName, err)
		return nil
	}

	result := []FormOption{}

	for _, entry := range tree.Entries {
		if entry.Path == nil {
			continue
		}

		path := *entry.Path
		basename := filepath.Base(path)
		if basename == "Tiltfile" || strings.HasSuffix(basename, ".tiltfile") {
			result = append(result, FormOption{
				Value: path,
				Name:  path,
			})
		}
	}

	return result
}
