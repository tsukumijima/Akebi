package main

import (
	"context"
	"crypto"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/mholt/acmez"
	"github.com/mholt/acmez/acme"
)

func renewCertificates() {
	for {
		client := &acmez.Client{Client: &acme.Client{}}
		client.ChallengeSolvers = solvers.GetDNSSolvers()
		err := renewCertificate(client, config.Certificate, config.MasterKey, "*."+config.Domain)
		if err != nil {
			log.Print(err)
		}

		if i := strings.IndexByte(config.API.Handler, '/'); i > 0 {
			hostname := config.API.Handler[:i]
			client.ChallengeSolvers = solvers.GetAPISolvers()
			err := renewCertificate(client, config.API.Certificate, config.API.Key, hostname)
			if err != nil {
				log.Print(err)
			} else if cert, err := loadCertificate(config.API.Certificate, config.API.Key, hostname); err != nil {
				log.Print(err)
			} else {
				httpCert.Lock()
				httpCert.Certificate = &cert
				httpCert.Unlock()
			}
		}

		time.Sleep(2*time.Hour + time.Duration(rand.Intn(60))*time.Minute)
	}
}

func renewCertificate(client *acmez.Client, certFile, keyFile, hostname string) error {
	cert, err := loadCertificate(certFile, keyFile, hostname)
	if err != nil {
		return err
	}

	if time.Until(cert.Leaf.NotAfter) > 30*24*time.Hour {
		return nil
	}

	key, ok := cert.PrivateKey.(crypto.Signer)
	if !ok {
		return fmt.Errorf("unexpected type %T", cert.PrivateKey)
	}

	acct, err := loadAccount(client)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Println("renewing the certificate for", hostname)
	return obtainCertificate(ctx, client, acct, key, certFile, hostname)
}
