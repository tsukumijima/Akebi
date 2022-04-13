package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/mholt/acmez"
)

var httpCert struct {
	sync.Mutex
	*tls.Certificate
}

func httpInit() (*http.Server, error) {
	cert, err := tls.LoadX509KeyPair(config.API.Certificate, config.API.Key)
	if os.IsNotExist(err) {
		cert.PrivateKey, err = loadKey(config.API.Key)
	} else if err == nil {
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	}
	if err != nil {
		return nil, err
	}
	httpCert.Certificate = &cert

	var cfg tls.Config
	cfg.NextProtos = []string{"h2", "http/1.1", acmez.ACMETLS1Protocol}

	cfg.GetCertificate = func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if chi.ServerName == "" {
			return nil, errors.New("missing server name")
		}
		if len(chi.SupportedProtos) == 1 && chi.SupportedProtos[0] == acmez.ACMETLS1Protocol {
			return solvers.GetTLSChallengeCert(chi.ServerName)
		}
		httpCert.Lock()
		defer httpCert.Unlock()
		if len(httpCert.Certificate.Certificate) == 0 {
			return getSelfSignedCert(httpCert.PrivateKey)
		}
		if err := chi.SupportsCertificate(httpCert.Certificate); err != nil {
			return nil, err
		}
		return httpCert.Certificate, nil
	}

	if config.API.ClientCA != "" {
		cert, err := ioutil.ReadFile(config.API.ClientCA)
		if err != nil {
			return nil, err
		}

		cfg.ClientCAs = x509.NewCertPool()
		cfg.ClientCAs.AppendCertsFromPEM(cert)
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	var mux http.ServeMux
	mux.Handle("/.well-known/acme-challenge/", http.HandlerFunc(solvers.HandleHTTPChallenge))
	mux.Handle(path.Clean(config.API.Handler+"/sign"), http.HandlerFunc(signingHandler))
	mux.Handle(path.Clean(config.API.Handler+"/certificate"), http.HandlerFunc(certificateHandler))

	server := http.Server{
		Handler:      &mux,
		TLSConfig:    &cfg,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  10 * time.Minute,
	}

	return &server, nil
}

func certificateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/pem-certificate-chain")
	http.ServeFile(w, r, config.Certificate)
}

func signingHandler(w http.ResponseWriter, r *http.Request) {
	sendError := func(status int) {
		http.Error(w, http.StatusText(status), status)
	}

	query := r.URL.Query()

	key, ok := privateKeys[query.Get("key")]
	if !ok {
		sendError(http.StatusNotFound)
		return
	}

	var hash crypto.Hash
	if h := query.Get("hash"); h != "" {
		for hash = crypto.MD4; ; hash++ {
			if hash > crypto.BLAKE2b_512 {
				sendError(http.StatusNotFound)
				return
			}
			if hash.String() == h && hash.Available() {
				// found
				break
			}
		}
	}

	var digest [65]byte
	n, err := io.ReadFull(r.Body, digest[:])
	if err != io.ErrUnexpectedEOF {
		sendError(http.StatusBadRequest)
		return
	}

	signature, err := key.Sign(rand.Reader, digest[:n], hash)
	if err != nil {
		sendError(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(signature)
}

func getSelfSignedCert(key crypto.PrivateKey) (*tls.Certificate, error) {
	pk, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", key)
	}

	template := x509.Certificate{
		SerialNumber:          &big.Int{},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, 1),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	data, err := x509.CreateCertificate(rand.Reader, &template, &template, &pk.PublicKey, pk)
	if err != nil {
		return nil, err
	}

	return &tls.Certificate{
		Certificate: [][]byte{data},
		PrivateKey:  key,
	}, nil
}
