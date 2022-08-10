package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/fatih/color"
)

// logging
func info_log(logs ...any) {
	now := time.Now()
	fmt.Print(now.Format("[2006/01/02 15:04:05] "), color.GreenString("INFO"), ":     ")
	for _, log := range logs {
		fmt.Print(log, " ")
	}
	fmt.Print("\n")
}
func error_log(logs ...any) {
	now := time.Now()
	fmt.Print(now.Format("[2006/01/02 15:04:05] "), color.RedString("ERROR"), ":    ")
	for _, log := range logs {
		fmt.Print(log, " ")
	}
	fmt.Print("\n")
}

func main() {

	// load config data
	if err := loadConfig(); err != nil {
		error_log("Configuration:", err)
		return
	}

	// suppress standard logger output
	log.SetOutput(ioutil.Discard)

	// setup reverse proxy
	var proxyPassURL, _ = url.Parse(config.ProxyPassURL)
	var reverseProxy = httputil.NewSingleHostReverseProxy(proxyPassURL)
	reverseProxy.ModifyResponse = func(response *http.Response) error {
		// set status code color
		var statusCodeText string
		switch {
		case response.StatusCode >= 200 && response.StatusCode <= 299:
			statusCodeText = color.GreenString(response.Status)
		case response.StatusCode >= 300 && response.StatusCode <= 399:
			statusCodeText = color.YellowString(response.Status)
		case response.StatusCode >= 400 && response.StatusCode <= 599:
			statusCodeText = color.RedString(response.Status)
		default:
			statusCodeText = response.Status
		}

		// print access log
		log := fmt.Sprintf(
			"%s - \"%s %s %s\" %s",
			response.Request.RemoteAddr,
			response.Request.Method,
			response.Request.URL.Path,
			response.Request.Proto,
			statusCodeText,
		)
		info_log(log)
		return nil
	}

	// setup TLS config
	var tlsConfig tls.Config
	if config.CustomCertificate.Certificate != "" && config.CustomCertificate.PrivateKey != "" {
		// use custom certificate
		tlsConfig = tls.Config{
			Certificates: []tls.Certificate{customCertificate},
		}
	} else {
		// use keyless server
		tlsConfig = tls.Config{
			// set GetCertificate callback
			GetCertificate: func() func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
				if config.MTLS.ClientCertificate != "" && config.MTLS.ClientCertificateKey != "" {
					// enable mTLS
					return GetKeylessServerCertificate(config.KeylessServerURL, mTLSCertificate)
				} else {
					// disable mTLS
					return GetKeylessServerCertificate(config.KeylessServerURL)
				}
			}(),
		}
	}

	// setup reverse proxy server
	var reverseProxyServer = http.Server{
		Addr:      config.ListenAddress,
		Handler:   reverseProxy,
		TLSConfig: &tlsConfig,
	}

	// serve reverse proxy
	go func() {
		info_log("Starting https reverse proxy server...")
		if config.CustomCertificate.Certificate != "" && config.CustomCertificate.PrivateKey != "" {
			info_log("Use custom https certificate and private key.")
		} else if config.MTLS.ClientCertificate != "" && config.MTLS.ClientCertificateKey != "" {
			info_log("Use mTLS client certificate and private key for " + config.KeylessServerURL + ".")
		}
		log := fmt.Sprintf("Listening on %s, Proxying %s.", config.ListenAddress, config.ProxyPassURL)
		info_log(log)
		var err = reverseProxyServer.ListenAndServeTLS("", "")
		if err != nil {
			error_log(err)
			return
		}
	}()

	// set signal trap
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)

	// When Ctrl+C is pressed
	<-quit
	defer info_log("Terminated https reverse proxy server.")
}
