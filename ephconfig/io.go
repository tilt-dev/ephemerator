package ephconfig

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

func ReadAllowlist() (*Allowlist, error) {
	asString := os.Getenv("ALLOWLIST")
	if asString == "" {
		return nil, fmt.Errorf("Missing env var ALLOWLIST")
	}

	allowlist := &Allowlist{}
	err := yaml.Unmarshal([]byte(asString), allowlist)
	if err != nil {
		return nil, fmt.Errorf("Reading ALLOWLIST: %v", err)
	}
	return allowlist, nil
}
