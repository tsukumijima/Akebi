package main

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type MTLSCertificates struct {
	ready                 bool
	ClientCAPool          *x509.CertPool
	ClientCertificatePair tls.Certificate
}

func GetKeylessServerCertificate(apiURL string, mTLSCertificates MTLSCertificates) func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
	apiURL = strings.TrimSuffix(apiURL, "/")

	var client *http.Client
	if mTLSCertificates.ready == false {
		client = http.DefaultClient
	} else {
		client = &http.Client{
			Transport: &http.Transport{
				Proxy:           http.ProxyFromEnvironment,
				IdleConnTimeout: 10 * time.Minute,
				TLSClientConfig: &tls.Config{
					RootCAs:      mTLSCertificates.ClientCAPool,
					Certificates: []tls.Certificate{mTLSCertificates.ClientCertificatePair},
				},
			},
			Timeout: 5 * time.Second,
		}
	}

	return func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		// require SNI
		if info.ServerName == "" {
			return nil, errors.New("fetching certificate: missing server name")
		}

		// fetch certificate
		res, err := client.Get(apiURL + "/certificate?" + url.QueryEscape(info.ServerName))
		if err != nil {
			return nil, fmt.Errorf("fetching certificate: %w", err)
		}
		defer res.Body.Close()

		if res.StatusCode != 200 {
			return nil, fmt.Errorf("fetching certificate: %s", res.Status)
		}

		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("fetching certificate: %w", err)
		}

		// decode certificate
		var cert tls.Certificate
		for {
			var block *pem.Block
			block, data = pem.Decode(data)
			if block == nil {
				break
			}
			if block.Type == "CERTIFICATE" {
				cert.Certificate = append(cert.Certificate, block.Bytes)
			}
		}

		if len(cert.Certificate) == 0 {
			return nil, errors.New("fetching certificate: no certificates returned")
		}

		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, fmt.Errorf("fetching certificate: %w", err)
		}

		der, err := x509.MarshalPKIXPublicKey(cert.Leaf.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("fetching certificate: %w", err)
		}

		hash := sha256.Sum256(der)
		cert.PrivateKey = Signer{
			pub:    cert.Leaf.PublicKey,
			id:     base64.RawURLEncoding.EncodeToString(hash[:]),
			api:    apiURL,
			client: client,
		}

		if err := info.SupportsCertificate(&cert); err != nil {
			return nil, fmt.Errorf("fetching certificate: %w", err)
		}

		return &cert, nil
	}
}

var _ crypto.Signer = Signer{}

type Signer struct {
	pub    crypto.PublicKey
	id     string
	api    string
	client *http.Client
}

func (signer Signer) Public() crypto.PublicKey {
	return signer.pub
}

func (signer Signer) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	hash := opts.HashFunc().String()

	res, err := signer.client.Post(
		signer.api+"/sign?key="+url.QueryEscape(signer.id)+"&hash="+url.QueryEscape(hash),
		"application/octet-stream", bytes.NewReader(digest))
	if err != nil {
		return nil, fmt.Errorf("signing digest: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("signing digest: %s", res.Status)
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("signing digest: %w", err)
	}

	return data, nil
}
