package config

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeResolver struct {
	addresses []net.IP
	err       error
	calls     int
}

func (resolver *fakeResolver) LookupIP(_ context.Context, network, _ string) ([]net.IP, error) {
	resolver.calls++
	if network != "ip4" {
		return nil, errors.New("unexpected network")
	}
	return resolver.addresses, resolver.err
}

func TestDefault(t *testing.T) {
	want := Config{
		ICMPTarget: "8.8.8.8",
		HTTPTarget: "https://www.google.com/generate_204",
	}
	if got := Default(); got != want {
		t.Fatalf("Default() = %#v, want %#v", got, want)
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	got, err := Load(context.Background(), filepath.Join(t.TempDir(), Filename))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != Default() {
		t.Fatalf("Load() = %#v, want %#v", got, Default())
	}
}

func TestLoadOverridesTargets(t *testing.T) {
	path := writeConfig(t, `{
		"icmpTarget": "1.1.1.1",
		"httpTarget": "https://example.com/health"
	}`)

	got, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := Config{ICMPTarget: "1.1.1.1", HTTPTarget: "https://example.com/health"}
	if got != want {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadAllowsPartialOverride(t *testing.T) {
	path := writeConfig(t, `{"httpTarget":"https://example.com/health"}`)

	got, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.ICMPTarget != DefaultICMPTarget {
		t.Fatalf("ICMPTarget = %q, want default %q", got.ICMPTarget, DefaultICMPTarget)
	}
	if got.HTTPTarget != "https://example.com/health" {
		t.Fatalf("HTTPTarget = %q", got.HTTPTarget)
	}
}

func TestLoadAcceptsResolvableHostname(t *testing.T) {
	path := writeConfig(t, `{"icmpTarget":"dns.google"}`)
	resolver := &fakeResolver{addresses: []net.IP{net.IPv4(8, 8, 8, 8)}}

	got, err := load(context.Background(), path, resolver)
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if got.ICMPTarget != "dns.google" {
		t.Fatalf("ICMPTarget = %q", got.ICMPTarget)
	}
	if resolver.calls != 1 {
		t.Fatalf("resolver calls = %d, want 1", resolver.calls)
	}
}

func TestLoadDoesNotResolveIPv4Literal(t *testing.T) {
	path := writeConfig(t, `{"icmpTarget":"1.1.1.1"}`)
	resolver := &fakeResolver{err: errors.New("must not be called")}

	if _, err := load(context.Background(), path, resolver); err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if resolver.calls != 0 {
		t.Fatalf("resolver calls = %d, want 0", resolver.calls)
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "empty document", content: "", want: "decode config"},
		{name: "malformed JSON", content: "{", want: "decode config"},
		{name: "unknown field", content: `{"extra":true}`, want: "unknown field"},
		{name: "multiple values", content: `{} {}`, want: "trailing data"},
		{name: "empty ICMP target", content: `{"icmpTarget":""}`, want: "icmpTarget"},
		{name: "ICMP whitespace", content: `{"icmpTarget":" 8.8.8.8"}`, want: "whitespace"},
		{name: "IPv6 ICMP target", content: `{"icmpTarget":"2001:4860:4860::8888"}`, want: "IPv4"},
		{name: "invalid hostname", content: `{"icmpTarget":"bad host"}`, want: "valid hostname"},
		{name: "empty HTTP target", content: `{"httpTarget":""}`, want: "httpTarget"},
		{name: "HTTP scheme", content: `{"httpTarget":"http://example.com"}`, want: "https"},
		{name: "HTTP without host", content: `{"httpTarget":"https:///health"}`, want: "host"},
		{name: "HTTP user info", content: `{"httpTarget":"https://user@example.com/health"}`, want: "user information"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfig(t, test.content)
			_, err := load(context.Background(), path, &fakeResolver{addresses: []net.IP{net.IPv4(8, 8, 8, 8)}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("load() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestLoadRejectsHostnameWithoutIPv4Address(t *testing.T) {
	path := writeConfig(t, `{"icmpTarget":"ipv6.example"}`)
	resolver := &fakeResolver{addresses: []net.IP{net.ParseIP("2001:db8::1")}}

	_, err := load(context.Background(), path, resolver)
	if err == nil || !strings.Contains(err.Error(), "did not resolve") {
		t.Fatalf("load() error = %v, want IPv4 resolution error", err)
	}
}

func TestLoadReportsResolverFailure(t *testing.T) {
	path := writeConfig(t, `{"icmpTarget":"unavailable.example"}`)
	resolver := &fakeResolver{err: errors.New("DNS unavailable")}

	_, err := load(context.Background(), path, resolver)
	if err == nil || !strings.Contains(err.Error(), "DNS unavailable") {
		t.Fatalf("load() error = %v, want resolver error", err)
	}
}

func TestConfigPathUsesExecutableDirectory(t *testing.T) {
	executable := filepath.Join("some", "directory", "obs-network-monitor.exe")
	want := filepath.Join("some", "directory", Filename)
	if got := configPath(executable); got != want {
		t.Fatalf("configPath() = %q, want %q", got, want)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), Filename)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
