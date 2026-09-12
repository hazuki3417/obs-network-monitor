package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	Filename          = "config.json"
	DefaultICMPTarget = "8.8.8.8"
	DefaultHTTPTarget = "https://www.google.com/generate_204"
)

type Config struct {
	ICMPTarget string `json:"icmpTarget"`
	HTTPTarget string `json:"httpTarget"`
}

type ipResolver interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
}

func Default() Config {
	return Config{
		ICMPTarget: DefaultICMPTarget,
		HTTPTarget: DefaultHTTPTarget,
	}
}

func LoadFromExecutable(ctx context.Context) (Config, error) {
	executable, err := os.Executable()
	if err != nil {
		return Config{}, fmt.Errorf("find executable: %w", err)
	}

	return load(ctx, configPath(executable), net.DefaultResolver)
}

func Load(ctx context.Context, path string) (Config, error) {
	return load(ctx, path, net.DefaultResolver)
}

func configPath(executable string) string {
	return filepath.Join(filepath.Dir(executable), Filename)
}

func load(ctx context.Context, path string, resolver ipResolver) (Config, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()

	result := Default()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return Config{}, fmt.Errorf("decode config %q: trailing data: %w", path, err)
	}

	if err := validate(ctx, result, resolver); err != nil {
		return Config{}, fmt.Errorf("validate config %q: %w", path, err)
	}

	return result, nil
}

func validate(ctx context.Context, value Config, resolver ipResolver) error {
	if err := validateICMPTarget(ctx, value.ICMPTarget, resolver); err != nil {
		return fmt.Errorf("icmpTarget: %w", err)
	}
	if err := validateHTTPTarget(value.HTTPTarget); err != nil {
		return fmt.Errorf("httpTarget: %w", err)
	}
	return nil
}

func validateICMPTarget(ctx context.Context, target string, resolver ipResolver) error {
	if target == "" {
		return errors.New("must not be empty")
	}
	if strings.TrimSpace(target) != target {
		return errors.New("must not contain leading or trailing whitespace")
	}

	if parsed := net.ParseIP(target); parsed != nil {
		if parsed.To4() == nil {
			return errors.New("must be an IPv4 address or hostname")
		}
		return nil
	}

	if !validHostname(target) {
		return errors.New("must be a valid hostname")
	}

	addresses, err := resolver.LookupIP(ctx, "ip4", target)
	if err != nil {
		return fmt.Errorf("resolve IPv4 address: %w", err)
	}
	for _, address := range addresses {
		if address.To4() != nil {
			return nil
		}
	}
	return errors.New("hostname did not resolve to an IPv4 address")
}

func validHostname(host string) bool {
	host = strings.TrimSuffix(host, ".")
	if host == "" || len(host) > 253 {
		return false
	}

	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') &&
				(char < 'A' || char > 'Z') &&
				(char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}

func validateHTTPTarget(target string) error {
	if target == "" {
		return errors.New("must not be empty")
	}
	if strings.TrimSpace(target) != target {
		return errors.New("must not contain leading or trailing whitespace")
	}

	parsed, err := url.ParseRequestURI(target)
	if err != nil {
		return fmt.Errorf("must be a valid URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return errors.New("scheme must be https")
	}
	if parsed.Host == "" {
		return errors.New("host is required")
	}
	if parsed.User != nil {
		return errors.New("user information is not allowed")
	}
	if parsed.Fragment != "" {
		return errors.New("fragment is not allowed")
	}
	return nil
}
