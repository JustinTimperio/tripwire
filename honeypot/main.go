package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	yaml "github.com/goccy/go-yaml"
	"gopkg.in/Graylog2/go-gelf.v1/gelf"
)

type config struct {
	GraylogAddr string `json:"graylog_addr"`
	HttpPorts   []int  `json:"http_ports"`
	UDPPorts    []int  `json:"udp_ports"`
	TCPPorts    []int  `json:"tcp_ports"`
}

func main() {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		panic(err)
	}

	var conf config
	err = yaml.Unmarshal(file, &conf)
	if err != nil {
		panic(err)
	}

	logs, err := gelf.NewWriter(conf.GraylogAddr)
	if err != nil {
		panic(err)
	}
	defer logs.Close()

	var Listeners []PortListener
	Listeners = append(Listeners, PortListener{
		Proto: "udp",
		Ports: conf.UDPPorts,
	})
	Listeners = append(Listeners, PortListener{
		Proto: "tcp",
		Ports: conf.TCPPorts,
	})

	ps := NewPortServer(Listeners)
	hs := NewHttpServer(conf.HttpPorts)

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		for stat := range ps.Stats {
			j, _ := json.Marshal(stat)
			log.Println(string(j))
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
			log.Println(string(j))
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
