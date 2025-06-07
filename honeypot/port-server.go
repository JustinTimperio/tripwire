package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type PortServer struct {
	Pls      []PortListener
	TLSCert  tls.Certificate
	CertPool *x509.CertPool

	Stats chan ConnectionStats
}

type PortListener struct {
	Proto string
	Port  int
	TLS   bool
}

func NewPortServer(portListeners []PortListener, cert tls.Certificate, certPool *x509.CertPool) *PortServer {
	return &PortServer{
		Pls:      portListeners,
		Stats:    make(chan ConnectionStats),
		TLSCert:  cert,
		CertPool: certPool,
	}
}

func (t *PortServer) Start() {
	var wg sync.WaitGroup

	for _, listener := range t.Pls {

		wg.Add(1)
		go func() {
			defer wg.Done()

			var (
				err  error
				conn net.Listener
			)

			switch listener.Proto {
			case "tcp":
				log.Printf("Listening on for %s traffic with the TLS flag %v on port: %v\n", listener.Proto, listener.TLS, listener.Port)

				if listener.TLS {
					var supported []uint16
					for _, cs := range tls.CipherSuites() {
						supported = append(supported, cs.ID)
					}

					// Create a TLS configuration
					config := &tls.Config{
						Certificates: []tls.Certificate{t.TLSCert},
						ClientAuth:   tls.NoClientCert, // Optional - for mutual TLS, use tls.RequireAndVerifyClientCert
						RootCAs:      t.CertPool,
						CipherSuites: supported,
					}
					conn, err = tls.Listen(listener.Proto, fmt.Sprintf(":%d", listener.Port), config)
					if err != nil {
						log.Printf("Error listening on TCP port %d: %v", listener.Port, err)
						return
					}

				} else {
					conn, err = net.Listen(listener.Proto, fmt.Sprintf(":%d", listener.Port))
					if err != nil {
						log.Printf("Error listening on TCP port %d: %v", listener.Port, err)
						return
					}
				}

				for {
					err = t.handleTCPConnection(conn)
					if err != nil {
						log.Println("Error Accepting Connection! error:", err)
					}
				}

			case "udp":
				conn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", listener.Port))
				if err != nil {
					log.Printf("Error listening on UDP port %d: %v", listener.Port, err)
					return
				}
				defer conn.Close()
				log.Printf("Listening on for %s traffic on port: %v\n", listener.Proto, listener.Port)

				for {
					t.handleUDPConnection(conn)
				}

			default:
				log.Printf("Unsupported protocol: %s on port %d", listener.Proto, listener.Port)
				return
			}
		}()
	}

	wg.Wait()
	log.Println("Port Server Stopped")
}

func (t *PortServer) handleTCPConnection(con net.Listener) error {
	conn, err := con.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()

	stats := ConnectionStats{
		StartTime: time.Now(),
	}

	host, port, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err == nil {
		stats.RemoteAddr = host
		stats.RemotePort = port
	} else {
		stats.RemoteAddr = conn.RemoteAddr().String()
		stats.RemotePort = "unknown"
	}

	host, port, err = net.SplitHostPort(conn.LocalAddr().String())
	if err == nil {
		stats.LocalAddr = host
		stats.LocalPort = port
	} else {
		stats.LocalAddr = conn.LocalAddr().String()
		stats.LocalPort = "unknown"
	}

	tlsConn, ok := conn.(*tls.Conn)
	if ok {
		tlsConn.Handshake()
		stats.TLS = tlsConn.ConnectionState()

		go func() {
			buffer := make([]byte, 1024)

			for {
				n, err := tlsConn.Read(buffer)
				if err != nil {
					stats.EndTime = time.Now()
					stats.Duration = stats.EndTime.Sub(stats.StartTime)
					t.Stats <- stats
					return
				}

				stats.DataVolume += n
			}
		}()

	} else {

		go func() {
			buffer := make([]byte, 1024)

			for {
				n, err := conn.Read(buffer)
				if err != nil {
					stats.EndTime = time.Now()
					stats.Duration = stats.EndTime.Sub(stats.StartTime)
					t.Stats <- stats
					return
				}

				stats.DataVolume += len(buffer[:n])
			}
		}()
	}

	return nil
}

func (t *PortServer) handleUDPConnection(conn net.PacketConn) {
	buffer := make([]byte, 1024)
	s := time.Now()
	n, addr, err := conn.ReadFrom(buffer)
	if err != nil {
		log.Printf("Error reading from UDP connection: %v", err)
		return
	}
	e := time.Now()

	// Capture basic stats. More comprehensive stats could be captured
	stats := ConnectionStats{
		StartTime:  s,
		EndTime:    e,
		Duration:   e.Sub(s),
		DataVolume: len(buffer[:n]),
	}

	host, port, err := net.SplitHostPort(addr.String())
	if err == nil {
		stats.RemoteAddr = host
		stats.RemotePort = port
	} else {
		stats.RemoteAddr = addr.String()
		stats.RemotePort = "unknown"
	}

	host, port, err = net.SplitHostPort(conn.LocalAddr().String())
	if err == nil {
		stats.LocalAddr = host
		stats.LocalPort = port
	} else {
		stats.LocalAddr = conn.LocalAddr().String()
		stats.LocalPort = "unknown"
	}
	t.Stats <- stats
}
