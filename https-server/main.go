package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"

	"github.com/fatih/color"
)

func main() {

	// log prefix (escape sequence)
	var infoLogPrefix = color.GreenString("Info") + ": "
	var errorLogPrefix = color.RedString("Error") + ":"

	// load config data
	if err := loadConfig(); err != nil {
		log.Fatalln(errorLogPrefix+" Configuration:", err)
	}

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
		log.Printf(
			"%s %s - \"%s %s %s\" %s",
			infoLogPrefix,
			response.Request.RemoteAddr,
			response.Request.Method,
			response.Request.URL.Path,
			response.Request.Proto,
			statusCodeText,
		)
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
		log.Println(infoLogPrefix, "Starting HTTPS reverse proxy server...")
		if config.CustomCertificate.Certificate != "" && config.CustomCertificate.PrivateKey != "" {
			log.Println(infoLogPrefix, "Use custom HTTPS certificate and private key.")
		} else if config.MTLS.ClientCertificate != "" && config.MTLS.ClientCertificateKey != "" {
			log.Println(infoLogPrefix, "Use mTLS client certificate and private key for "+config.KeylessServerURL+".")
		}
		log.Printf("%s Listening on %s, Proxing %s.", infoLogPrefix, config.ListenAddress, config.ProxyPassURL)
		var err = reverseProxyServer.ListenAndServeTLS("", "")
		if err != nil {
			log.Fatalln(errorLogPrefix, err)
			return
		}
	}()

	// set signal trap
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)

	// When Ctrl+C is pressed
	<-quit
	defer log.Println(infoLogPrefix, "Terminated HTTPS reverse proxy server.")
}
