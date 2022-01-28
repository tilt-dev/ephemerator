package ephconfig

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

func ReadAllowlist() (*Allowlist, error) {
	asString := os.Getenv("EPH_ALLOWLIST")
	if asString == "" {
		return nil, fmt.Errorf("Missing env var EPH_ALLOWLIST")
	}

	allowlist := &Allowlist{}
	err := yaml.Unmarshal([]byte(asString), allowlist)
	if err != nil {
		return nil, fmt.Errorf("Reading EPH_ALLOWLIST: %v", err)
	}
	return allowlist, nil
}

func ReadGatewayHost() (string, error) {
	asString := os.Getenv("EPH_GATEWAY_HOST")
	if asString == "" {
		return "", fmt.Errorf("Missing env var EPH_GATEWAY_HOST")
	}
	return asString, nil
}
