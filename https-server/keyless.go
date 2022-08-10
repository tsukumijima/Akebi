package main

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func GetKeylessServerCertificate(apiURL string, mTLSCertificate ...tls.Certificate) func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
	apiURL = strings.TrimSuffix(apiURL, "/")

	var client *http.Client
	if len(mTLSCertificate) == 0 {
		client = http.DefaultClient
	} else {
		client = &http.Client{
			Transport: &http.Transport{
				Proxy:           http.ProxyFromEnvironment,
				IdleConnTimeout: 10 * time.Minute,
				TLSClientConfig: &tls.Config{
					Certificates: mTLSCertificate,
				},
			},
			Timeout: 5 * time.Second,
		}
	}

	return func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		// require SNI
		if info.ServerName == "" {
			error_log("Fetching certificate: missing server name")
			return nil, nil
		}

		// fetch certificate
		res, err := client.Get(apiURL + "/certificate?" + url.QueryEscape(info.ServerName))
		if err != nil {
			log := fmt.Sprintf("Fetching certificate: %s", err)
			error_log(log)
			return nil, err
		}
		defer res.Body.Close()

		if res.StatusCode != 200 {
			log := fmt.Sprintf("Fetching certificate: keyless api server returned returned http error: %s", res.Status)
			error_log(log)
			return nil, nil
		}

		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log := fmt.Sprintf("Fetching certificate: %s", err)
			error_log(log)
			return nil, err
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
			error_log("Fetching certificate: no certificates returned")
			return nil, nil
		}

		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			log := fmt.Sprintf("Fetching certificate: %s", err)
			error_log(log)
			return nil, err
		}

		der, err := x509.MarshalPKIXPublicKey(cert.Leaf.PublicKey)
		if err != nil {
			log := fmt.Sprintf("Fetching certificate: %s", err)
			error_log(log)
			return nil, err
		}

		// initialize Signer
		hash := sha256.Sum256(der)
		cert.PrivateKey = Signer{
			pub:    cert.Leaf.PublicKey,
			id:     base64.RawURLEncoding.EncodeToString(hash[:]),
			api:    apiURL,
			client: client,
		}

		if err := info.SupportsCertificate(&cert); err != nil {
			log := fmt.Sprintf("Fetching certificate: %s", err)
			error_log(log)
			return nil, err
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

	// send signing request
	// "key" parameter is Base64-encoded SHA-256 hash of the certificate in DER format.
	// "hash" parameter is string that represents the type of hash function (e.g. SHA-256)
	// example of URL: https://akebi.example.com/sign?key=pyrfaV5udNlWgp5ZSSSHVRd8nDQ5yp8ILTiU_CVXmRk&hash=SHA-256
	res, err := signer.client.Post(
		signer.api+"/sign?key="+url.QueryEscape(signer.id)+"&hash="+url.QueryEscape(hash),
		"application/octet-stream", bytes.NewReader(digest))
	if err != nil {
		log := fmt.Sprintf("Signing digest: %s", err)
		error_log(log)
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		log := fmt.Sprintf("Signing digest: %s", err)
		error_log(log)
		return nil, err
	}

	// read the signature
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log := fmt.Sprintf("Signing digest: %s", err)
		error_log(log)
		return nil, err
	}

	info_log("Obtained signature from keyless api server.")

	return data, nil
}
