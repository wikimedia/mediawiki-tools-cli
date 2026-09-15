package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCheckHostRunningTrue(t *testing.T) {
	origLookup := dnsLookupHost
	origTimeout := dnsLookupTimeout
	t.Cleanup(func() {
		dnsLookupHost = origLookup
		dnsLookupTimeout = origTimeout
	})

	dnsLookupTimeout = 50 * time.Millisecond
	dnsLookupHost = func(ctx context.Context, hostname string) ([]string, error) {
		return []string{"10.0.0.2"}, nil
	}

	if !checkHost("mysql") {
		t.Fatal("expected checkHost to return true for resolved service")
	}
}

func TestCheckHostRunningFalseOnLookupError(t *testing.T) {
	origLookup := dnsLookupHost
	origTimeout := dnsLookupTimeout
	t.Cleanup(func() {
		dnsLookupHost = origLookup
		dnsLookupTimeout = origTimeout
	})

	dnsLookupTimeout = 50 * time.Millisecond
	dnsLookupHost = func(ctx context.Context, hostname string) ([]string, error) {
		return nil, errors.New("lookup failed")
	}

	if checkHost("missing") {
		t.Fatal("expected checkHost to return false when lookup errors")
	}
}

func TestCheckHostRespectsTimeout(t *testing.T) {
	origLookup := dnsLookupHost
	origTimeout := dnsLookupTimeout
	t.Cleanup(func() {
		dnsLookupHost = origLookup
		dnsLookupTimeout = origTimeout
	})

	dnsLookupTimeout = 10 * time.Millisecond
	dnsLookupHost = func(ctx context.Context, hostname string) ([]string, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	start := time.Now()
	if checkHost("slow-service") {
		t.Fatal("expected checkHost to return false on timeout")
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("expected timeout to be fast, elapsed=%s", elapsed)
	}
}
