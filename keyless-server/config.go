package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

var config struct {
	Domain     string `json:"domain"`     // required
	Nameserver string `json:"nameserver"` // required
	CName      string `json:"cname"`      // optional

	Certificate string `json:"certificate"` // required, file path
	MasterKey   string `json:"master_key"`  // required, file path
	LegacyKeys  string `json:"legacy_keys"` // optional, file glob

	API struct {
		Handler     string `json:"handler"`     // required
		Certificate string `json:"certificate"` // required, file path
		Key         string `json:"key"`         // required, file path
		ClientCA    string `json:"client_ca"`   // optional, file path
	} `json:"api"`

	LetsEncrypt struct {
		Account    string `json:"account"`     // required, file path
		AccountKey string `json:"account_key"` // required, file path
	} `json:"letsencrypt"`

	Replica string `json:"replica"` // optional
}

func loadConfig() error {
	f, err := os.Open("config.json")
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return fmt.Errorf("config.json: %w", err)
	}

	// check required fields
	if config.Domain == "" {
		return errors.New("domain is not configured")
	}
	if config.Nameserver == "" {
		return errors.New("nameserver is not configured")
	}
	if config.Certificate == "" {
		return errors.New("certificate file path is not configured")
	}
	if config.MasterKey == "" {
		return errors.New("master_key file path is not configured")
	}
	if config.API.Handler == "" {
		return errors.New("api.handler is not configured")
	}
	if config.API.Certificate == "" {
		return errors.New("api.certificate file path is not configured")
	}
	if config.API.Key == "" {
		return errors.New("api.key file path is not configured")
	}
	if config.LetsEncrypt.Account == "" {
		return errors.New("letsencrypt.account file path is not configured")
	}
	if config.LetsEncrypt.AccountKey == "" {
		return errors.New("letsencrypt.account_key file path is not configured")
	}

	return dnsConfig()
}
