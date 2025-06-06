package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

var maxRecivedBuffers = 10_000

type PortServer struct {
	Pls   []PortListener
	Stats chan ConnectionStats
}

type PortListener struct {
	Proto string
	Ports []int
}

type ConnectionStats struct {
	RemoteAddr string        `json:"remote_addr"`
	LocalAddr  string        `json:"local_addr"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    time.Time     `json:"end_time"`
	Duration   time.Duration `json:"duration"`
	DataVolume int           `json:"data_volume"`
	Hostname   string        `json:"hostname"`
}

func NewPortServer(portListeners []PortListener) *PortServer {
	return &PortServer{
		Pls:   portListeners,
		Stats: make(chan ConnectionStats),
	}
}

func (t *PortServer) Start() {
	var wg sync.WaitGroup

	for _, listener := range t.Pls {

		for _, port := range listener.Ports {

			wg.Add(1)
			go func(port int, proto string) {
				defer wg.Done()

				switch proto {
				case "tcp":
					conn, err := net.Listen(proto, fmt.Sprintf(":%d", port))
					if err != nil {
						log.Printf("Error listening on TCP port %d: %v", port, err)
						return
					}

					log.Printf("Listening on for %s traffic on port: %v\n", proto, port)
					for {
						stats, err := t.handleTCPConnection(conn)
						if err != nil {
							log.Println("Error Accepting Connection! error:", err)
						}
						t.Stats <- *stats
					}

				case "udp":
					pconn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", port))
					if err != nil {
						log.Printf("Error listening on UDP port %d: %v", port, err)
						return
					}
					defer pconn.Close()

					log.Printf("Listening on for %s traffic on port: %v\n", proto, port)
					t.handleUDPConnection(pconn)

				default:
					log.Printf("Unsupported protocol: %s on port %d", proto, port)
					return
				}

			}(port, listener.Proto)
		}
	}

	wg.Wait()
	log.Println("Port Server Stopped")
}

func (t *PortServer) handleTCPConnection(con net.Listener) (*ConnectionStats, error) {
	conn, err := con.Accept()
	if err != nil {
		return &ConnectionStats{}, err
	}
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	localAddr := conn.LocalAddr().String()

	log.Printf("New connection from %s to %s\n", remoteAddr, localAddr)

	// Capture basic stats.  More comprehensive stats could be captured
	stats := ConnectionStats{
		RemoteAddr: remoteAddr,
		LocalAddr:  localAddr,
		StartTime:  time.Now(),
	}

	go func() {
		buffer := make([]byte, 4096)
		for range maxRecivedBuffers {
			n, err := conn.Read(buffer)
			if err != nil {
				stats.EndTime = time.Now()
				stats.Duration = stats.EndTime.Sub(stats.StartTime)
				log.Printf("Connection closed after %s from %s to %s\n", stats.Duration, remoteAddr, localAddr)
				return
			}

			log.Printf("Received data from %s to %s", remoteAddr, localAddr)

			// Update statistics
			stats.DataVolume += len(buffer[:n])
		}

		go conn.Close()
		stats.EndTime = time.Now()
		stats.Duration = stats.EndTime.Sub(stats.StartTime)
		log.Printf("Closed after %s from %s to %s\n", stats.Duration, remoteAddr, localAddr)
	}()

	return &stats, nil
}

func (t *PortServer) handleUDPConnection(pconn net.PacketConn) {
	hn, _ := os.Hostname()
	buffer := make([]byte, 4096)
	for {
		s := time.Now()
		n, addr, err := pconn.ReadFrom(buffer)
		if err != nil {
			log.Printf("Error reading from UDP connection: %v", err)
			return
		}
		e := time.Now()

		remoteAddr := addr.String()
		log.Printf("Received %d bytes from UDP %s", n, remoteAddr)

		// Capture basic stats. More comprehensive stats could be captured
		stats := ConnectionStats{
			RemoteAddr: remoteAddr,
			LocalAddr:  pconn.LocalAddr().String(),
			StartTime:  s,
			EndTime:    e,
			Duration:   e.Sub(s),
			DataVolume: len(buffer[:n]),
			Hostname:   hn,
		}
		t.Stats <- stats

		log.Printf("UDP received from %s, duration %s", remoteAddr, stats.Duration)
	}
}
