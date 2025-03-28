package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	cflog "github.com/cloudflare/cfssl/log"
)

func init() {
	cflog.Level = cflog.LevelWarning
}

func main() {
	flag.StringVar(&Config.Cert, "cert", "cert.pem", "proxy CA cert")
	flag.StringVar(&Config.Key, "key", "key.pem", "proxy CA key")
	flag.StringVar(&Config.Addr, "addr", "", "proxy listen host")
	flag.StringVar(&Config.Port, "port", "8080", "proxy listen port")
	flag.StringVar(&Config.BrowserProfile, "browser", "chrome133", "browser profile to emulate (chrome133, firefox117, safari18_0, etc.)")
	flag.StringVar(&Config.Upstream, "upstream", "", "upstream SOCKS5 proxy in format host:port:user:pass or host:port")
	flag.BoolVar(&Config.Debug, "debug", false, "enable debug")
	flag.Parse()

	if Config.Debug {
		cflog.Level = cflog.LevelDebug
	}

	if !fileExists(Config.Cert) || !fileExists(Config.Key) {
		if fileExists(Config.Cert) {
			log.Println("found CA cert, but no corresponding key")
			os.Exit(-1)
		} else if fileExists(Config.Key) {
			log.Println("found CA key, but no corresponding cert")
			os.Exit(-1)
		}

		log.Println("CA cert and key do not exist, generating")
		err := generateCA()
		if err != nil {
			log.Fatal("Failed generating CA", err)
		}
	}

	loadExistingCA()
	generateSessionKey()

	// Initialize the TLS client
	err := initTLSClient()
	if err != nil {
		log.Println("Warning: Failed to initialize TLS client:", err)
	}

	// Setup custom dialer for upstream proxy
	CustomDialer, err = NewUpstreamDialer(Config.Upstream, time.Second*10)
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr: Config.Addr + ":" + Config.Port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleTunneling(w, r)
			} else {
				handleHTTP(w, r)
			}
		}),
	}

	fmt.Printf(
		"HTTP Proxy Server listen at %s:%s, with browser profile: %s\n",
		Config.Addr, Config.Port, Config.BrowserProfile,
	)

	if Config.Upstream != "" {
		fmt.Printf("Using upstream SOCKS5 proxy: %s\n", Config.Upstream)
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
		os.Exit(-1)
	}
}
