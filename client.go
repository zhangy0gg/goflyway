package goflyway

import (
	. "github.com/coyove/goflyway/v"
	"github.com/coyove/tcpmux/toh"
	kcp "github.com/xtaci/kcp-go"

	"net"
	"net/http"
)

type ClientConfig struct {
	commonConfig
	Upstream  string
	Bind      string
	WebSocket bool
	VPN       bool
}

func NewClient(localaddr string, config *ClientConfig) error {
	config.check()

	tr := *http.DefaultTransport.(*http.Transport)
	tr.MaxConnsPerHost = 100
	tr.Dial = func(network string, address string) (net.Conn, error) {
		switch {
		case config.KCP:
			return kcp.Dial(address)
		case config.VPN:
			return vpnDial(address)
		default:
			return net.Dial(network, address)
		}
	}

	dialer := toh.NewDialer(config.Key, config.Upstream,
		toh.WithWebSocket(config.WebSocket),
		toh.WithInactiveTimeout(config.Timeout),
		toh.WithTransport(&tr))

	mux, err := net.Listen("tcp", localaddr)
	if err != nil {
		return err
	}

	for {
		conn, err := mux.Accept()
		if err != nil {
			return err
		}

		go func(conn net.Conn) {
			downconn := toh.NewBufConn(conn)
			defer conn.Close()

			var up net.Conn
			var err error

			if config.KCP {
				up, err = kcp.Dial(config.Upstream)
			} else {
				up, err = dialer.Dial()
			}

			if err != nil {
				Vprint(err)
				return
			}
			defer up.Close()

			upconn := toh.NewBufConn(up)
			if _, err := upconn.Write([]byte(config.Bind + "\n")); err != nil {
				Vprint(err)
				return
			}

			resp, err := upconn.ReadBytes('\n')
			if err != nil || string(resp) != "OK\n" {
				Vprint(err, string(resp))
				return
			}

			Bridge(downconn, upconn, nil)
		}(conn)
	}
}