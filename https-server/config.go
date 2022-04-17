package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"muzzammil.xyz/jsonc"
)

var config struct {
	ListenAddress    string `json:"listen_address"`     // required, example: 0.0.0.0:7000
	ProxyPassURL     string `json:"proxy_pass_url"`     // required, example: http://localhost:7001/
	KeylessServerURL string `json:"keyless_server_url"` // required, example: https://akebi.example.com/

	MTLS struct {
		ClientCertificate    string `json:"client_certificate"`     // optional, file path
		ClientCertificateKey string `json:"client_certificate_key"` // optional, file path
	} `json:"mtls"`
}

var mTLSCertificate tls.Certificate

func loadConfig() error {
	path, err := os.Executable()
	f, err := ioutil.ReadFile(filepath.Dir(path) + "/akebi-https-server.json")
	if err != nil {
		return err
	}

	if err := jsonc.Unmarshal(f, &config); err != nil {
		return fmt.Errorf("akebi-https-server.json: %w", err)
	}

	// check required fields
	if config.ListenAddress == "" {
		return errors.New("listen_address is not configured.")
	}
	if config.ProxyPassURL == "" {
		return errors.New("proxy_pass_url is not configured.")
	}
	if config.KeylessServerURL == "" {
		return errors.New("keyless_server_url is not configured.")
	}

	// load mTLS certificate
	if config.MTLS.ClientCertificate != "" && config.MTLS.ClientCertificateKey != "" {
		// load client certificate pair
		mTLSCertificate, err = tls.LoadX509KeyPair(config.MTLS.ClientCertificate, config.MTLS.ClientCertificateKey)
		if err != nil {
			return fmt.Errorf("could not open certificate file: %w", err)
		}
	}

	return err
}
