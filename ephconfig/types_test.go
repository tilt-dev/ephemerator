package ephconfig

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var allowlist = &Allowlist{
	RepoBase:  "tilt-dev",
	RepoNames: []string{"tilt-avatars", "tilt-example-html"},
}

type allowedCase struct {
	spec EnvSpec
	msg  string
}

func TestAllowed(t *testing.T) {
	cases := []allowedCase{
		{spec: EnvSpec{"tilt-dev/tilt-avatars", "main", "Tiltfile"}, msg: ""},
		{spec: EnvSpec{"tilt-dev/tilt-avatars2", "main", "Tiltfile"}, msg: "unrecognized repo name"},
		{spec: EnvSpec{"tilt-dev2/tilt-avatars", "main", "Tiltfile"}, msg: "unrecognized base"},
		{spec: EnvSpec{"tilt-dev/tilt-avatars", "main", "/Tiltfile"}, msg: "path must be relative"},
		{spec: EnvSpec{"tilt-dev/tilt-avatars", "main", "x/../../Tiltfile"}, msg: "no '..' references"},
		{spec: EnvSpec{"tilt-dev/tilt-avatars", "m x", "Tiltfile"}, msg: "malformed branch"},
		{spec: EnvSpec{"tilt-dev/tilt-avatars", "main", "Tilt file"}, msg: "malformed path"},
	}

	for i, c := range cases {
		c := c
		t.Run(fmt.Sprintf("TestAllowed%d", i), func(t *testing.T) {
			err := IsAllowed(allowlist, c.spec)
			if c.msg == "" {
				assert.NoError(t, err)
			} else {
				if assert.Error(t, err) {
					assert.Contains(t, err.Error(), c.msg)
				}
			}
		})
	}
}
