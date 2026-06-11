package cmd

import (
	"fmt"
	"net/url"
	"strconv"

	"charm.land/huh/v2"
)

// runInteractiveForm prompts for the proxy settings when --target is missing.
// The pointers arrive pre-filled with flag defaults so the form shows them as
// initial values (huh has no separate default mechanism).
func runInteractiveForm(target, port, origin, name *string) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Backend URL").
				Description("Backend the proxy will forward requests to").
				Placeholder("http://localhost:8080").
				Value(target).
				Validate(validateTargetURL),
			huh.NewInput().
				Title("Proxy port").
				Description("Port corsproxy listens on").
				Value(port).
				Validate(validatePort),
			huh.NewInput().
				Title("Allowed origin").
				Description(`"*" = allow all, or a specific IP/origin (e.g. 192.168.1.5)`).
				Value(origin).
				Validate(validateOrigin),
			huh.NewInput().
				Title("Name (optional)").
				Description("Label this server for resume (leave empty to use a generated id)").
				Value(name),
		),
	)
	return form.Run()
}

func validateTargetURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("must be http://host:port, e.g. http://localhost:8080")
	}
	return nil
}

func validatePort(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("port must be a number between 1 and 65535")
	}
	return nil
}

func validateOrigin(s string) error {
	if s == "" {
		return fmt.Errorf(`cannot be empty — use "*" to allow all origins`)
	}
	return nil
}
