package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {

	// load config data
	if err := loadConfig(); err != nil {
		log.Fatalln("configuration:", err)
	}

	// setup reverse proxy
	var url, _ = url.Parse(config.ProxyPassURL)
	var reverseProxy = http.Server{
		Addr:    config.ListenAddress,
		Handler: httputil.NewSingleHostReverseProxy(url),
		// set GetCertificate callback
		TLSConfig: &tls.Config{
			GetCertificate: GetKeylessServerCertificate(config.KeylessServerURL),
		},
	}

	// serve reverse proxy
	var err = reverseProxy.ListenAndServeTLS("", "")
	if err != nil {
		log.Fatalln("error:", err)
	}
}
