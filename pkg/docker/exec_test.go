package docker

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestExecDrainsOutputAfterProcessExit(t *testing.T) {
	for _, exitCode := range []int{0, 23} {
		t.Run(fmt.Sprintf("exit_%d", exitCode), func(t *testing.T) {
			inspected := make(chan struct{}, 1)
			releaseOutput := make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(releaseOutput) }) }
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1.45/containers/test-container/exec":
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"Id":"test-exec"}`)
				case "/v1.45/exec/test-exec/start":
					io.Copy(io.Discard, r.Body)
					r.Body.Close()
					conn, rw, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					defer conn.Close()
					fmt.Fprint(rw, "HTTP/1.1 101 UPGRADED\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n")
					fmt.Fprint(rw, "initial output\n")
					rw.Flush()
					// Model output still in transit when Docker reports process exit.
					<-releaseOutput
					fmt.Fprint(rw, "FOO=bar\nfinal output\n")
					rw.Flush()
				case "/v1.45/exec/test-exec/json":
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, `{"Running":false,"ExitCode":%d}`, exitCode)
					select {
					case inspected <- struct{}{}:
					default:
					}
				default:
					t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)
			t.Cleanup(release)
			t.Setenv("DOCKER_HOST", strings.Replace(server.URL, "http://", "tcp://", 1))
			t.Setenv("DOCKER_API_VERSION", "1.45")

			stdout, output := execTestPipe(t)
			stdin, input := execTestPipe(t)
			previousStdin, previousStdout := os.Stdin, os.Stdout
			os.Stdin, os.Stdout = stdin, output
			t.Cleanup(func() { os.Stdin, os.Stdout = previousStdin, previousStdout })
			// Keep stdin open: completion must wait for output, not for stdin EOF.
			defer input.Close()
			returned := make(chan int, 1)
			go func() {
				returned <- Exec("test-container", ExecOptions{Command: []string{"env"}, Env: []string{"FOO=bar"}})
			}()

			select {
			case <-inspected:
			case <-time.After(5 * time.Second):
				t.Fatal("exec never inspected the completed process")
			}
			select {
			case code := <-returned:
				t.Fatalf("exec returned %d before its output stream was drained", code)
			case <-time.After(150 * time.Millisecond):
			}
			release()
			select {
			case code := <-returned:
				if code != exitCode {
					t.Fatalf("exit code = %d, want %d", code, exitCode)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("exec did not return after output EOF with stdin still open")
			}
			output.Close()
			got, err := io.ReadAll(stdout)
			if err != nil {
				t.Fatal(err)
			}
			if want := "initial output\nFOO=bar\nfinal output\n"; string(got) != want {
				t.Fatalf("output = %q, want %q", got, want)
			}
		})
	}
}

func execTestPipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	return r, w
}
