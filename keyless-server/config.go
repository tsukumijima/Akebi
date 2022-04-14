package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"muzzammil.xyz/jsonc"
)

var config struct {
	Domain     string `json:"domain"`     // required
	Nameserver string `json:"nameserver"` // required
	CName      string `json:"cname"`      // optional

	Certificate string `json:"certificate"` // required, file path
	MasterKey   string `json:"master_key"`  // required, file path
	LegacyKeys  string `json:"legacy_keys"` // optional, file glob

	KeylessAPI struct {
		Handler     string `json:"handler"`     // required
		Certificate string `json:"certificate"` // required, file path
		PrivateKey  string `json:"private_key"` // required, file path
		ClientCA    string `json:"client_ca"`   // optional, file path
	} `json:"keyless_api"`

	LetsEncrypt struct {
		Account    string `json:"account"`     // required, file path
		AccountKey string `json:"account_key"` // required, file path
	} `json:"letsencrypt"`

	Replica string `json:"replica"` // optional
}

func loadConfig() error {
	path, err := os.Executable()
	f, err := ioutil.ReadFile(filepath.Dir(path) + "/akebi-keyless-server.json")
	if err != nil {
		return err
	}

	if err := jsonc.Unmarshal(f, &config); err != nil {
		return fmt.Errorf("akebi-keyless-server.json: %w", err)
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
	if config.KeylessAPI.Handler == "" {
		return errors.New("keyless_api.handler is not configured")
	}
	if config.KeylessAPI.Certificate == "" {
		return errors.New("keyless_api.certificate file path is not configured")
	}
	if config.KeylessAPI.PrivateKey == "" {
		return errors.New("keyless_api.private_key file path is not configured")
	}
	if config.LetsEncrypt.Account == "" {
		return errors.New("letsencrypt.account file path is not configured")
	}
	if config.LetsEncrypt.AccountKey == "" {
		return errors.New("letsencrypt.account_key file path is not configured")
	}

	return dnsConfig()
}
