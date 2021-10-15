package ports

import (
	"net"
	"testing"

	"github.com/alecthomas/assert"
)

func Test_FreeUpFrom(t *testing.T) {
	type test struct {
		loopLimit     int
		requestedPort string
		boundPort     string
		resultingPort string
	}

	defaultStartingPort = "56664"

	tests := []test{
		// 56665 is only probably free...
		{loopLimit: 25, requestedPort: "56665", boundPort: "", resultingPort: "56665"},
		{loopLimit: 25, requestedPort: "56665", boundPort: "56665", resultingPort: "56666"},
		{loopLimit: 25, requestedPort: "default to be used", boundPort: "", resultingPort: "56664"},
		{loopLimit: 1, requestedPort: "default to be used", boundPort: defaultStartingPort, resultingPort: "panic"},
	}

	for _, tc := range tests {
		portSearchLoopsBeforePanic = tc.loopLimit
		ln, lnErr := net.Listen("tcp", ":"+tc.boundPort)

		if tc.resultingPort != "panic" {
			resultingPort := FreeUpFrom(tc.requestedPort)
			if resultingPort != tc.resultingPort {
				t.Errorf("Expected %s, got %s", tc.resultingPort, resultingPort)
			}
		} else {
			assert.Panics(t, func() { FreeUpFrom(tc.requestedPort) })
		}

		if lnErr != nil {
			ln.Close()
		}
	}
}

func TestValidity_isValid_IsValidAndFree(t *testing.T) {
	type test struct {
		valid bool
		port  string
	}

	tests := []test{
		{valid: true, port: "1"},
		{valid: true, port: "8080"},
		{valid: true, port: "65535"},
		{valid: false, port: "foo"},
		{valid: false, port: "99999999"},
		{valid: false, port: "-1"},
	}

	for _, tc := range tests {
		errorOrNil := isValid(tc.port)
		if !tc.valid && errorOrNil == nil {
			t.Errorf("Expected error for port %s", tc.port)
		}
	}
	for _, tc := range tests {
		errorOrNil := IsValidAndFree(tc.port)
		if !tc.valid && errorOrNil == nil {
			t.Errorf("Expected error for port %s", tc.port)
		}
	}
}
