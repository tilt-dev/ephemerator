package ephconfig

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Format of the Allowlist key in ephctrl-allowlist ConfigMap
type Allowlist struct {
	RepoBase string `json:"repoBase" yaml:"repoBase"`

	RepoNames []string `json:"repoNames" yaml:"repoNames"`
}

type EnvSpec struct {
	Repo   string
	Branch string
	Path   string
}

// Validate the environment spec for anything that looks suspicious:
// - The repo must match our allow list.
// - The path must be a valid relative path.
// - The branch must look like a reasonable branch name.
func IsAllowed(allowlist *Allowlist, spec EnvSpec) error {
	err := isRepoAllowed(allowlist, spec.Repo)
	if err != nil {
		return err
	}

	err = isBranchAllowed(spec.Branch)
	if err != nil {
		return err
	}

	return isPathAllowed(spec.Path)
}

func isRepoAllowed(allowlist *Allowlist, repo string) error {
	parts := strings.Split(repo, "/")
	if len(parts) < 2 {
		return fmt.Errorf("Forbidden: malformed repo: %s", repo)
	}

	repoBase := strings.Join(parts[:len(parts)-1], "/")
	repoName := parts[len(parts)-1]
	if repoBase != allowlist.RepoBase {
		return fmt.Errorf("Forbidden: unrecognized base: %s", repo)
	}

	isAllowed := false
	for _, n := range allowlist.RepoNames {
		if n == repoName {
			isAllowed = true
		}
	}

	if !isAllowed {
		return fmt.Errorf("Forbidden: unrecognized repo name: %s", repo)
	}

	return nil
}

var branchRe = regexp.MustCompile("^[a-zA-Z][a-zA-Z_/0-9-]*$")

func isBranchAllowed(branch string) error {
	if !branchRe.MatchString(branch) {
		return fmt.Errorf("Forbidden: malformed branch name")
	}
	return nil
}

var pathRe = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z_/0-9.-]*$")

func isPathAllowed(path string) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("Forbidden: path must be relative")
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("Forbidden: no '..' references allowed in path")
	}
	if !pathRe.MatchString(path) {
		return fmt.Errorf("Forbidden: malformed path")
	}
	return nil
}
