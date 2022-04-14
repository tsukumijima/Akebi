package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/acmez"
	"github.com/mholt/acmez/acme"
)

// Checks that the server is setup correctly.
// Load private keys (master and legacy) into memory.
// Rotating the master key requires a process restart.
func checkSetup() error {
	_, err := loadAccount(nil)
	if err != nil {
		return err
	}
	if err := loadAPI(); err != nil {
		return err
	}
	return loadCertificateAndKeys()
}

// Sets the server up interactively.
func interactiveSetup() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
	fmt.Println("Running setup...")

	if err := loadConfig(); err != nil {
		log.Fatalln("Error:", err)
	}
	if err := checkSetup(); err == nil {
		fmt.Println("It seems you're all set!")
		return
	}

	fmt.Println()
	ctx := context.Background()
	client := &acmez.Client{Client: &acme.Client{}}

	acct, err := setupAccount(ctx, client)
	if err != nil {
		log.Fatalln("Error:", err)
	}

	if err := setupCertificateAndKeys(ctx, client, acct); err != nil {
		log.Fatalln("Error:", err)
	}

	if err := setupAPI(ctx, client, acct); err != nil {
		log.Fatalln("Error:", err)
	}

	fmt.Println()
	fmt.Println("Done!")
}

func setupAccount(ctx context.Context, client *acmez.Client) (acct acme.Account, err error) {
	acct, err = loadAccount(client)
	if err == nil {
		fmt.Println("Using the existing Let's Encrypt account.")
		return acct, nil
	}

	fmt.Println("Creating a new Let's Encrypt account...")

	err = os.MkdirAll(filepath.Dir(config.LetsEncrypt.Account), 0700)
	if err != nil {
		return acct, err
	}

	acct.PrivateKey, err = setupKey("account", config.LetsEncrypt.AccountKey)
	if err != nil {
		return acct, err
	}

	var answer string

	fmt.Println()
	fmt.Print("Accept Let's Encrypt ToS? [y/n]: ")
	if n, _ := fmt.Scanln(&answer); n != 1 || answer != "y" {
		return acct, errors.New("did not accept Let's Encrypt ToS")
	}

	fmt.Print("Use the Let's Encrypt production API? [y/n]: ")
	if n, _ := fmt.Scanln(&answer); n != 1 || answer != "y" {
		client.Directory = letsencryptStaging + "directory"
	} else {
		client.Directory = letsencryptProduction + "directory"
	}

	fmt.Print("Enter an email address: ")
	if n, _ := fmt.Scanln(&answer); n == 1 && answer != "" {
		acct.Contact = append(acct.Contact, "mailto:"+answer)
	}
	fmt.Println()

	acct.TermsOfServiceAgreed = true
	acct, err = client.NewAccount(ctx, acct)
	if err != nil {
		return acct, err
	}

	json, err := json.MarshalIndent(acct, "", "  ")
	if err != nil {
		return acct, err
	}

	err = ioutil.WriteFile(config.LetsEncrypt.Account, json, 0400)
	return acct, err
}

func setupCertificateAndKeys(ctx context.Context, client *acmez.Client, acct acme.Account) error {
	if loadCertificateAndKeys() == nil {
		fmt.Println("Using the existing certificate and keys.")
		return nil
	}

	nameserver := config.Nameserver
	app := filepath.Base(os.Args[0])

	key, err := setupKey("master", config.MasterKey)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Starting DNS server for domain validation...")
	fmt.Println("Please, ensure that:")
	fmt.Printf(" - NS records for %s point to %s\n", config.Domain, nameserver)
	fmt.Printf(" - %s is reachable from the internet on UDP %s:53\n", app, nameserver)
	fmt.Print("Continue? ")
	fmt.Scanln()

	conn, err := setupUDP(nameserver, ":53")
	if err != nil {
		return err
	}
	defer conn.Close()
	go dnsServe(conn)

	fmt.Println()
	client.ChallengeSolvers = solvers.GetDNSSolvers()
	fmt.Printf("Obtaining a certificate for *.%s...\n", config.Domain)
	return obtainCertificate(ctx, client, acct, key, config.Certificate, "*."+config.Domain)
}

func setupAPI(ctx context.Context, client *acmez.Client, acct acme.Account) error {
	if loadAPI() == nil {
		fmt.Println("Using the existing Keyless API certificates and key.")
		return nil
	}

	var hostname string
	app := filepath.Base(os.Args[0])
	if i := strings.IndexByte(config.KeylessAPI.Handler, '/'); i > 0 {
		hostname = config.KeylessAPI.Handler[:i]
	} else {
		return errors.New("Keyless API handler does not have a hostname.")
	}

	key, err := setupKey("Keyless API", config.KeylessAPI.PrivateKey)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Starting HTTPS server for hostname validation...")
	fmt.Println("Please, ensure that:")
	fmt.Printf(" - %s is reachable from the internet on TCP %s:443\n", app, hostname)
	fmt.Print("Continue? ")
	fmt.Scanln()

	ln, err := setupTCP(hostname, ":443")
	if err != nil {
		return err
	}
	defer ln.Close()

	server, err := httpInit()
	if err != nil {
		return err
	}
	defer server.Close()

	go server.ServeTLS(ln, "", "")

	fmt.Println()
	client.ChallengeSolvers = solvers.GetAPISolvers()
	fmt.Printf("Obtaining a certificate for %s...\n", hostname)
	return obtainCertificate(ctx, client, acct, key, config.KeylessAPI.Certificate, hostname)
}

func setupKey(keyName, keyFile string) (*ecdsa.PrivateKey, error) {
	if buf, err := ioutil.ReadFile(keyFile); os.IsNotExist(err) {
		fmt.Println("Creating a new", keyName, "private key...")

		err := os.MkdirAll(filepath.Dir(keyFile), 0700)
		if err != nil {
			return nil, err
		}

		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}

		der, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return nil, err
		}

		pem := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
		err = ioutil.WriteFile(keyFile, pem, 0400)
		if err != nil {
			return nil, err
		}

		return key, nil

	} else {
		fmt.Println("Using the existing", keyName, "private key...")
		if err != nil {
			return nil, err
		}

		blk, _ := pem.Decode(buf)
		if blk == nil {
			return nil, errors.New("no PEM data found")
		}
		return x509.ParseECPrivateKey(blk.Bytes)
	}
}

func setupUDP(host, port string) (net.PacketConn, error) {
	conn, _ := net.ListenPacket("udp", host+port)
	if conn == nil && host != "" {
		conn, _ = net.ListenPacket("udp", port)
	}
	if conn != nil {
		return conn, nil
	}

	fmt.Println()
	fmt.Printf("Could not listen on UDP %s.\n", port)
	addr, err := setupAddress()
	if err != nil {
		return nil, err
	}

	return net.ListenPacket("udp", addr)
}

func setupTCP(host, port string) (net.Listener, error) {
	conn, _ := net.Listen("tcp", host+port)
	if conn == nil && host != "" {
		conn, _ = net.Listen("tcp", port)
	}
	if conn != nil {
		return conn, nil
	}

	fmt.Println()
	fmt.Printf("Could not listen on TCP %s.\n", port)
	addr, err := setupAddress()
	if err != nil {
		return nil, err
	}

	return net.Listen("tcp", addr)
}

func setupAddress() (string, error) {
	fmt.Print("Enter the host:port address to listen on: ")

	var answer string
	fmt.Scanln(&answer)
	host, port, err := net.SplitHostPort(answer)
	return host + ":" + port, err
}
