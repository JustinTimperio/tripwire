package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	yaml "github.com/goccy/go-yaml"
	"gopkg.in/Graylog2/go-gelf.v1/gelf"
)

type ConnectionStats struct {
	// Common
	RemoteAddr string `json:"remote_addr"`
	RemotePort string `json:"remote_port"`
	LocalAddr  string `json:"local_addr"`
	LocalPort  string `json:"local_port"`
	DataVolume int    `json:"data_volume"`

	// Port Server
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`

	// Http Server
	Method     string              `json:"method"`
	RequestURI string              `json:"request_uri"`
	Headers    http.Header         `json:"headers"`
	UserAgent  string              `json:"user_agent"`
	PostForm   url.Values          `json:"post_form"`
	TLS        tls.ConnectionState `json:"tls"`
}

type Config struct {
	GraylogAddr string         `json:"graylog_addr"`
	HttpPorts   []HttpServer   `json:"http_servers"`
	ListenPorts []PortListener `json:"port_servers"`
}

func main() {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		panic(err)
	}

	var conf Config
	err = yaml.Unmarshal(file, &conf)
	if err != nil {
		panic(err)
	}

	logs, err := gelf.NewWriter(conf.GraylogAddr)
	if err != nil {
		panic(err)
	}
	defer logs.Close()

	certBytes, keyBytes, err := generateSelfSignedCert()
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("server.crt", certBytes, 0600)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("server.key", keyBytes, 0600)
	if err != nil {
		panic(err)
	}

	cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
	if err != nil {
		panic(err)
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(certBytes)

	ps := NewPortServer(conf.ListenPorts, cert, certPool)
	hs := NewHttpServer(conf.HttpPorts, cert, certPool)

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		for stat := range ps.Stats {
			j, _ := json.Marshal(stat)
			_, err := logs.Write(j)
			if err != nil {
				log.Println("Error! Failed to write log to graylog", err)
			}
		}
	}()

	go func() {
		defer wg.Done()
		for request := range hs.Stats {
			j, _ := json.Marshal(request)
			_, err := logs.Write(j)
			if err != nil {
				log.Println("Error! Failed to write log to graylog", err)
			}
		}

	}()

	go ps.Start()
	go hs.Start()
	wg.Wait()

}
