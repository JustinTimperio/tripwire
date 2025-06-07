package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
)

type HttpServerPool struct {
	Stats    chan ConnectionStats
	TLSCert  tls.Certificate
	CertPool *x509.CertPool
	Servers  []HttpServer
}

type HttpServer struct {
	Port int  `json:"port"`
	TLS  bool `json:"tls"`
}

func NewHttpServer(servers []HttpServer, cert tls.Certificate, certPool *x509.CertPool) HttpServerPool {
	return HttpServerPool{
		Stats:    make(chan ConnectionStats),
		TLSCert:  cert,
		CertPool: certPool,
		Servers:  servers,
	}
}

func (hs *HttpServerPool) Start() {
	var wg sync.WaitGroup

	for _, server := range hs.Servers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if server.TLS {
				log.Printf("Listening on for HTTP traffic with TLS on port: %v\n", server.Port)
				var supported []uint16
				for _, cs := range tls.CipherSuites() {
					supported = append(supported, cs.ID)
				}

				config := &tls.Config{
					Certificates: []tls.Certificate{hs.TLSCert},
					ClientAuth:   tls.NoClientCert, // Optional - for mutual TLS, use tls.RequireAndVerifyClientCert
					RootCAs:      hs.CertPool,
					CipherSuites: supported,
				}

				httpServ := &http.Server{
					Addr:      fmt.Sprintf(":%d", server.Port),
					Handler:   http.HandlerFunc(hs.handleIndex),
					TLSConfig: config,
				}

				err := httpServ.ListenAndServeTLS("server.crt", "server.key")
				if err != nil {
					log.Fatalf("failed to start TLS server on port %d: %v", server.Port, err)
					return
				}

			} else {
				log.Printf("Listening on for HTTP traffic without TLS on port: %v\n", server.Port)
				err := http.ListenAndServe(fmt.Sprintf(":%d", server.Port), http.HandlerFunc(hs.handleIndex))
				if err != nil {
					log.Fatalf("failed to start non-TLS server on port %d: %v", server.Port, err)
				}
			}

		}()
	}

	wg.Wait()
	log.Println("Http Server Stopped")
}
func (hs *HttpServerPool) handleIndex(w http.ResponseWriter, r *http.Request) {
	stats := ConnectionStats{}

	host, port, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		stats.RemoteAddr = host
		stats.RemotePort = port
	} else {
		stats.RemoteAddr = r.RemoteAddr
		stats.RemotePort = "unknown"

	}

	host, port, err = net.SplitHostPort(fmt.Sprintf("%v", r.Context().Value(http.LocalAddrContextKey)))
	if err == nil {
		stats.LocalAddr = host
		stats.LocalPort = port
	} else {
		stats.RemoteAddr = r.RemoteAddr
		stats.RemotePort = "unknown"
	}

	stats.DataVolume = int(r.ContentLength)
	r.ParseForm()
	stats.PostForm = r.PostForm
	stats.Method = r.Method
	stats.RequestURI = r.RequestURI
	stats.Headers = r.Header
	stats.UserAgent = r.UserAgent()
	if r.TLS != nil {
		stats.TLS = *r.TLS
	}

	hs.Stats <- stats
}
