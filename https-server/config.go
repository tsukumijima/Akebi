package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

var config struct {
	ListenAddress    string `json:"listen_address"`     // required, example: 0.0.0.0:7000
	ProxyPassURL     string `json:"proxy_pass_url"`     // required, example: http://localhost:7001/
	KeylessServerURL string `json:"keyless_server_url"` // required, example: https://akebi.example.com/
}

func loadConfig() error {
	f, err := os.Open("akebi-https-server.json")
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return fmt.Errorf("config.json: %w", err)
	}

	// check required fields
	if config.ListenAddress == "" {
		return errors.New("listen_address is not configured")
	}
	if config.ProxyPassURL == "" {
		return errors.New("proxy_pass_url is not configured")
	}
	if config.KeylessServerURL == "" {
		return errors.New("keyless_server_url is not configured")
	}

	return err
}
