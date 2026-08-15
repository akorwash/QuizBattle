package main

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"testing"
)

func TestRunHealthcheck(t *testing.T) {
	const release = "0123456789abcdef0123456789abcdef01234567"
	for name, testCase := range map[string]struct {
		status          int
		responseStatus  string
		responseRelease string
		expectedRelease string
		wantError       bool
	}{
		"healthy": {
			status: http.StatusOK, responseStatus: "ok", responseRelease: release, expectedRelease: release,
		},
		"unhealthy status code": {
			status: http.StatusServiceUnavailable, responseStatus: "unavailable", wantError: true,
		},
		"release mismatch": {
			status: http.StatusOK, responseStatus: "ok", responseRelease: release, expectedRelease: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", wantError: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/healthz" {
					t.Errorf("unexpected healthcheck path: %s", r.URL.Path)
				}
				w.WriteHeader(testCase.status)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"status": testCase.responseStatus, "release": testCase.responseRelease,
				})
			})}
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(func() { _ = server.Close() })

			port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
			err = runHealthcheck(port, testCase.expectedRelease)
			if !testCase.wantError && err != nil {
				t.Fatalf("healthy endpoint failed: %v", err)
			}
			if testCase.wantError && err == nil {
				t.Fatal("unhealthy endpoint passed")
			}
		})
	}
}

func TestRunHealthcheckRejectsInvalidPort(t *testing.T) {
	if err := runHealthcheck("not-a-port", ""); err == nil {
		t.Fatal("invalid healthcheck port was accepted")
	}
}
