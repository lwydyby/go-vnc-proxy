package proxy

import (
	"github.com/pkg/errors"
	"io"
	"lwydyby/go-vnc-proxy/conf"
	"net"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

// peer represents a vnc proxy peer
// with a websocket connection and a vnc backend connection
type peer struct {
	source *websocket.Conn
	target net.Conn
}

func NewPeer(ws *websocket.Conn, addr string) (*peer, error) {
	if ws == nil {
		return nil, errors.New("websocket connection is nil")
	}
	var (
		c   net.Conn
		err error
	)
	c, err = net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to vnc backend")
	}

	err = c.(*net.TCPConn).SetKeepAlive(true)
	if err != nil {
		return nil, errors.Wrap(err, "enable vnc backend connection keepalive failed")
	}

	err = c.(*net.TCPConn).SetKeepAlivePeriod(30 * time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "set vnc backend connection keepalive period failed")
	}
	if conf.Conf.AppInfo.TLS {
		c, err = Connect(strings.Split(addr, ":")[0], ws, c)
		if err != nil {
			return nil, err
		}
	}
	return &peer{
		source: ws,
		target: c,
	}, nil
}

// ReadSource copy source stream to target connection
func (p *peer) ReadSource() error {
	if _, err := io.Copy(p.target, p.source); err != nil {
		return errors.Wrapf(err, "copy source(%v) => target(%v) failed", p.source.RemoteAddr(), p.target.RemoteAddr())
	}
	return nil
}

// ReadTarget copys target stream to source connection
func (p *peer) ReadTarget() error {
	if _, err := io.Copy(p.source, p.target); err != nil {
		return errors.Wrapf(err, "copy target(%v) => source(%v) failed", p.target.RemoteAddr(), p.source.RemoteAddr())
	}
	return nil
}

// Close close the websocket connection and the vnc backend connection
func (p *peer) Close() {
	p.source.Close()
	p.target.Close()
}
