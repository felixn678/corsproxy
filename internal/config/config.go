// Package config holds the proxy configuration and its validation rules.
package config

import (
	"fmt"
	"net/url"
)

// Config is the validated runtime configuration of the proxy.
type Config struct {
	Target *url.URL // backend URL that requests are forwarded to
	Port   int      // port the proxy listens on
	Origin string   // "*" to allow every origin, or a specific host/origin
}

// New parses and validates raw input values into a Config.
func New(target string, port int, origin string) (*Config, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("invalid target: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("target must be http://host:port or https://host:port, e.g. http://localhost:8080 (got %q)", target)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("target is missing a host, e.g. http://localhost:8080 (got %q)", target)
	}

	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535 (got %d)", port)
	}

	if origin == "" {
		return nil, fmt.Errorf(`origin cannot be empty — use "*" to allow all, or a specific address like 192.168.1.5`)
	}

	return &Config{Target: u, Port: port, Origin: origin}, nil
}
