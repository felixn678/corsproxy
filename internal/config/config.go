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
		return nil, fmt.Errorf("target không hợp lệ: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("target phải dạng http://host:port hoặc https://host:port, ví dụ http://localhost:8080 (nhận được %q)", target)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("target thiếu host, ví dụ http://localhost:8080 (nhận được %q)", target)
	}

	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port phải trong khoảng 1-65535 (nhận được %d)", port)
	}

	if origin == "" {
		return nil, fmt.Errorf(`origin không được rỗng — dùng "*" để cho phép tất cả, hoặc một địa chỉ cụ thể như 192.168.1.5`)
	}

	return &Config{Target: u, Port: port, Origin: origin}, nil
}
