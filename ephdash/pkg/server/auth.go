package server

import (
	"fmt"
)

type AuthSettings struct {
	FakeUser string
	Proxy    string
}

func (s AuthSettings) Validate() error {
	if s.FakeUser == "" && s.Proxy == "" {
		return fmt.Errorf("Auth settings missing. Please specify --auth-fake-user or --auth-proxy")
	} else if s.FakeUser != "" && s.Proxy != "" {
		return fmt.Errorf("Cannot specify both --auth-fake-user or --auth-proxy")
	}
	return nil
}

type AuthResponse struct {
	User  string `json:"user"`
	Email string `json:"email"`
}
