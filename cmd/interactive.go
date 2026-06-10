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
func runInteractiveForm(target, port, origin *string) error {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Backend URL").
				Description("Backend mà proxy sẽ forward request đến").
				Placeholder("http://localhost:8080").
				Value(target).
				Validate(validateTargetURL),
			huh.NewInput().
				Title("Proxy port").
				Description("Port mà corsproxy lắng nghe").
				Value(port).
				Validate(validatePort),
			huh.NewInput().
				Title("Allowed origin").
				Description(`"*" = cho phép tất cả, hoặc IP/origin cụ thể (vd 192.168.1.5)`).
				Value(origin).
				Validate(validateOrigin),
		),
	)
	return form.Run()
}

func validateTargetURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("cần URL dạng http://host:port, vd http://localhost:8080")
	}
	return nil
}

func validatePort(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("port phải là số trong khoảng 1-65535")
	}
	return nil
}

func validateOrigin(s string) error {
	if s == "" {
		return fmt.Errorf(`không được rỗng — dùng "*" để cho phép tất cả`)
	}
	return nil
}
