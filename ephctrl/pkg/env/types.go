package env

import (
	"fmt"
	"strings"
)

// Format of the Allowlist key in ephctrl-allowlist ConfigMap
type Allowlist struct {
	RepoBase string `json:"repoBase" yaml:"repoBase"`

	RepoNames []string `json:"repoNames" yaml:"repoNames"`
}

func IsAllowed(allowlist *Allowlist, repo string) error {
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
