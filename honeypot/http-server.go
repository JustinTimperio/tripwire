package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

type HttpServer struct {
	Ports []int
	Stats chan Record
}

type Record struct {
	RemoteAddr string      `json:"remote_addr"`
	Method     string      `json:"method"`
	RequestURI string      `json:"request_uri"`
	Headers    http.Header `json:"headers"`
	UserAgent  string      `json:"user_agent"`
	PostForm   url.Values  `json:"post_form"`
	EventTime  uint64      `json:"event_time"`
	Hostname   string      `json:"hostname"`
}

func NewHttpServer(ports []int) HttpServer {
	return HttpServer{
		Ports: ports,
		Stats: make(chan Record),
	}
}

func (hs *HttpServer) Start() {

	var wg sync.WaitGroup
	http.HandleFunc("/", hs.handleIndex)

	for _, port := range hs.Ports {
		wg.Add(1)
		log.Printf("Listening on for http traffic on port: %v\n", port)

		go func() {
			defer wg.Done()
			http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		}()
	}

	wg.Wait()
	log.Println("Http Server Stopped")
}

func (hs *HttpServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	hn, _ := os.Hostname()

	data := Record{}
	data.RemoteAddr = r.RemoteAddr
	data.Method = r.Method
	data.RequestURI = r.RequestURI
	data.Headers = r.Header
	data.UserAgent = r.UserAgent()
	r.ParseForm()
	data.PostForm = r.PostForm
	data.EventTime = uint64(time.Now().Unix())
	data.Hostname = hn

	hs.Stats <- data
}
