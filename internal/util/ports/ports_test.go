package ports

import (
	"errors"
	"net"
	"reflect"
	"strconv"
	"testing"

	"github.com/alecthomas/assert"
)

func Test_FreeUpFrom(t *testing.T) {
	originalDefault, originalLimit := defaultStartingPort, portSearchLoopsBeforePanic
	t.Cleanup(func() {
		defaultStartingPort, portSearchLoopsBeforePanic = originalDefault, originalLimit
	})
	defaultStartingPort = "56664"
	for _, tc := range []struct {
		name, requested, want string
		limit                 int
		busy                  map[string]bool
		checked               []string
	}{
		{name: "requested port available", requested: "56665", want: "56665", limit: 25, checked: []string{"56665"}},
		{name: "one occupied port", requested: "56665", want: "56666", limit: 25, busy: map[string]bool{"56665": true}, checked: []string{"56665", "56666"}},
		{name: "multiple occupied ports", requested: "56665", want: "56667", limit: 25, busy: map[string]bool{"56665": true, "56666": true}, checked: []string{"56665", "56666", "56667"}},
		{name: "invalid port uses default", requested: "default to be used", want: "56664", limit: 25, checked: []string{"56664"}},
		{name: "limit exhausted", requested: "default to be used", want: "panic", limit: 1, busy: map[string]bool{"56664": true}, checked: []string{"56664"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			portSearchLoopsBeforePanic = tc.limit
			var checked []string
			available := func(port string) error {
				checked = append(checked, port)
				if tc.busy[port] {
					return errors.New("occupied fixture port")
				}
				return nil
			}
			if tc.want == "panic" {
				assert.Panics(t, func() { freeUpFrom(tc.requested, available) })
			} else if got := freeUpFrom(tc.requested, available); got != tc.want {
				t.Errorf("Expected %s, got %s", tc.want, got)
			}
			if !reflect.DeepEqual(checked, tc.checked) {
				t.Errorf("probed %v, want %v", checked, tc.checked)
			}
		})
	}
}

func TestFreeUpFromOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	if err := IsValidAndFree(port); err == nil {
		t.Fatal("bound port reported free")
	}
	originalLimit := portSearchLoopsBeforePanic
	t.Cleanup(func() { portSearchLoopsBeforePanic = originalLimit })
	portSearchLoopsBeforePanic = 1
	assert.Panics(t, func() { FreeUpFrom(port) })
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
