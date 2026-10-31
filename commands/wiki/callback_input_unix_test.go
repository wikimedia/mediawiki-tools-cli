//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package wiki

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
	"golang.org/x/term"
)

func TestHiddenCallbackInputCancelsWithoutConsumingNextPrompt(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	a, err := wikiauth.Begin(wikiauth.DefaultClientID, wikiauth.DefaultRedirectURI)
	if err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.SetIn(slave)
	cmd.SetErr(io.Discard)
	input, err := startCallbackInput(cmd, a)
	if err != nil || input == nil {
		t.Fatal("could not start terminal input", err)
	}
	stopped := make(chan struct{})
	go func() { input.stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("hidden reader did not cancel")
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("terminal mode was not restored", err)
	}
	if _, err := master.Write([]byte("next-prompt\n")); err != nil {
		t.Fatal(err)
	}
	got := make(chan string, 1)
	go func() { data := make([]byte, 64); n, _ := slave.Read(data); got <- string(data[:n]) }()
	select {
	case text := <-got:
		if text != "next-prompt\n" {
			t.Fatalf("next prompt input was stolen: %q", text)
		}
	case <-time.After(time.Second):
		t.Fatal("next prompt could not read terminal")
	}
}

func TestRunningBrowserLoginAcceptsPastedCallback(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	redirect := "http://" + listener.Addr().String() + "/auth/login"
	_ = listener.Close()
	a, err := wikiauth.Begin(wikiauth.DefaultClientID, redirect)
	if err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.SetIn(slave)
	cmd.SetErr(slave)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	callback := redirect + "?" + url.Values{"code": {"fixture-code"}, "state": {a.State}}.Encode()
	written := make(chan error, 1)
	observed := make(chan string, 1)
	go func() {
		data := make([]byte, 512)
		var output string
		for !strings.Contains(output, "Callback URL (paste and press Enter): ") {
			n, err := master.Read(data)
			if err != nil {
				written <- err
				return
			}
			output += string(data[:n])
		}
		_, err := master.Write([]byte(callback + "\r"))
		written <- err
		remainder, _ := io.ReadAll(master)
		observed <- output + string(remainder)
	}()
	got, err := authorizeWiki(ctx, cmd, a, false, true)
	if err != nil || got != callback {
		t.Fatalf("pasted callback failed: %v", err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	_ = slave.Close()
	select {
	case output := <-observed:
		if strings.Contains(output, "fixture-code") {
			t.Fatal("terminal echoed the callback authorization code")
		}
	case <-ctx.Done():
		t.Fatal("terminal output did not finish")
	}
}

func TestBrowserCallbackStopsHiddenInput(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	redirect := "http://" + listener.Addr().String() + "/auth/login"
	_ = listener.Close()
	a, err := wikiauth.Begin(wikiauth.DefaultClientID, redirect)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := term.GetState(int(slave.Fd()))
	cmd := &cobra.Command{}
	cmd.SetIn(slave)
	cmd.SetErr(io.Discard)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	callback := redirect + "?" + url.Values{"code": {"fixture-code"}, "state": {a.State}}.Encode()
	done := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: time.Second}
		for ctx.Err() == nil {
			response, err := client.Get(callback)
			if err == nil {
				response.Body.Close()
				done <- nil
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		done <- fmt.Errorf("callback listener did not start")
	}()
	got, err := authorizeWiki(ctx, cmd, a, false, true)
	if err != nil || got != callback {
		t.Fatal("browser callback failed", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	after, _ := term.GetState(int(slave.Fd()))
	if !reflect.DeepEqual(before, after) {
		t.Fatal("browser callback left terminal in raw mode")
	}
}
