// This program implements a TCP service.
//
// Upon receiving a new TCP connection, the service runs
// a zerotrace traceroute to the peer.  Once the traceroute
// completes, the service establishes a TLS connection with
// the peer.  After that, the connection is closed.

package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"net/netip"
	"os"

	"github.com/brave/zerotrace"
)

var l = log.New(os.Stderr, "tlssvc: ", log.Ldate|log.Lmicroseconds|log.LUTC|log.Lshortfile)

type tcpHandler func(net.Conn)

// handleConns accepts new incoming TCP connections.
func handleConns(addr string, handle tcpHandler) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		l.Fatalf("Error listening: %v", err)
	}

	l.Print("Waiting for new TCP connetions...")
	for {
		conn, err := ln.Accept()
		if err != nil {
			l.Printf("Error accepting TCP connection: %v", err)
			continue
		}
		go handle(conn)
	}
}

// getTCPHandler returns a function that first initiates a zerotrace traceroute
// to the peer and -- once that completes -- finishes the TLS handshake and
// closes the connection.
func getTCPHandler(config *tls.Config, iface string, port uint16) tcpHandler {
	ztConfig := zerotrace.NewDefaultConfig()
	ztConfig.Interface = iface
	zt := zerotrace.NewZeroTrace(ztConfig)
	if err := zt.Start(); err != nil {
		l.Fatalf("Error starting zerotrace: %v", err)
	}

	return func(conn net.Conn) {
		defer conn.Close()

		// We must run the zerotrace measurement *before* the TLS handshake
		// because Atlas probes are going to terminate the connection as soon
		// as the fetched the server certificate.
		l.Printf("Starting traceroute to new peer: %s", conn.RemoteAddr())
		duration, err := zt.CalcRTT(conn)
		if err != nil {
			l.Printf("Error running ZeroTrace: %v", err)
			return
		}
		l.Printf("measurement,%s,%d\n", conn.RemoteAddr(), duration.Microseconds())

		tlsConn := tls.Server(conn, config)
		if err = tlsConn.Handshake(); err != nil {
			if !errors.Is(err, io.EOF) {
				l.Printf("Error finishing TLS handshake: %v", err)
			}
			return
		}
		l.Printf("Finished TLS handshake with %s.", conn.RemoteAddr())
		tlsConn.Close()
	}
}

func main() {
	var (
		certFile string
		keyFile  string
		iface    string
		addr     string
		log      string
	)
	flag.StringVar(&certFile, "cert", "", "The TLS server's certificate file.")
	flag.StringVar(&keyFile, "key", "", "The TLS server's key file.")
	flag.StringVar(&iface, "iface", "", "The networking interface to use zerotrace for.")
	flag.StringVar(&addr, "addr", "0.0.0.0:443", "The TLS server's address to listen on.")
	flag.StringVar(&log, "log", "", "The log file to which stdout is written to.")
	flag.Parse()

	if certFile == "" || keyFile == "" || iface == "" {
		l.Fatalf("The flags -cert, -key, and -iface must be provided.")
	}
	if log != "" {
		f, err := os.OpenFile(log, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			l.Fatalf("Error opening log file: %v", err)
		}
		defer f.Close()
		l.SetOutput(io.MultiWriter(os.Stdout, f))
	}

	addrPort := netip.MustParseAddrPort(addr)

	// Build our TLS configuration.
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		l.Fatalf("Error loading key pair: %v", err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Start accepting new TCP connections.
	handleConns(addr, getTCPHandler(
		tlsConfig,
		iface,
		addrPort.Port(),
	))
}
