package main

import (
	"crypto/tls"
	"errors"
	"flag"
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

	CustomCertificate struct {
		Certificate string `json:"certificate"` // optional, file path
		PrivateKey  string `json:"private_key"` // optional, file path
	} `json:"custom_certificate"`
}

var mTLSCertificate tls.Certificate
var customCertificate tls.Certificate

func loadConfig() error {
	path, err := os.Executable()
	f, err := ioutil.ReadFile(filepath.Dir(path) + "/akebi-https-server.json")
	if err == nil {
		if err := jsonc.Unmarshal(f, &config); err != nil {
			return fmt.Errorf("akebi-https-server.json: %w", err)
		}
	}
	err = nil // reset

	// parse arguments
	argument1 := flag.String("listen-address", "", "Address that HTTPS server listens on.\nSpecify 0.0.0.0:port to listen on all interfaces.")
	argument2 := flag.String("proxy-pass-url", "", "URL of HTTP server to reverse proxy.")
	argument3 := flag.String("keyless-server-url", "", "URL of HTTP server to reverse proxy.")
	argument4 := flag.String("mtls-client-certificate", "", "Optional: Client certificate of mTLS for akebi.example.com (Keyless API).")
	argument5 := flag.String("mtls-client-certificate-key", "", "Optional: Client private key of mTLS for akebi.example.com (Keyless API).")
	argument6 := flag.String("custom-certificate", "", "Optional: Use your own HTTPS certificate instead of Akebi Keyless Server.")
	argument7 := flag.String("custom-private-key", "", "Optional: Use your own HTTPS private key instead of Akebi Keyless Server.")
	flag.Parse()

	// set arguments
	if *argument1 != "" {
		config.ListenAddress = *argument1
	}
	if *argument2 != "" {
		config.ProxyPassURL = *argument2
	}
	if *argument3 != "" {
		config.KeylessServerURL = *argument3
	}
	if *argument4 != "" {
		config.MTLS.ClientCertificate = *argument4
	}
	if *argument5 != "" {
		config.MTLS.ClientCertificateKey = *argument5
	}
	if *argument6 != "" {
		config.CustomCertificate.Certificate = *argument6
	}
	if *argument7 != "" {
		config.CustomCertificate.PrivateKey = *argument7
	}

	// check required fields
	if config.ListenAddress == "" {
		return errors.New("--listen-address (json:listen_address) is not configured.")
	}
	if config.ProxyPassURL == "" {
		return errors.New("--proxy-pass-url (json:proxy_pass_url) is not configured.")
	}
	if config.KeylessServerURL == "" {
		return errors.New("--keyless-server-url (json:keyless_server_url) is not configured.")
	}

	// load mTLS certificate
	if config.MTLS.ClientCertificate != "" && config.MTLS.ClientCertificateKey != "" {
		// load client certificate pair
		mTLSCertificate, err = tls.LoadX509KeyPair(config.MTLS.ClientCertificate, config.MTLS.ClientCertificateKey)
		if err != nil {
			return fmt.Errorf("could not open certificate file: %w", err)
		}
	}

	// load custom certificate
	if config.CustomCertificate.Certificate != "" && config.CustomCertificate.PrivateKey != "" {
		// load certificate pair
		customCertificate, err = tls.LoadX509KeyPair(config.CustomCertificate.Certificate, config.CustomCertificate.PrivateKey)
		if err != nil {
			return fmt.Errorf("could not open certificate file: %w", err)
		}
	}

	return err
}
