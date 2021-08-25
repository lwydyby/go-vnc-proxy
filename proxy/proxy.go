package proxy

import (
	log "github.com/lwydyby/logrus"
	"golang.org/x/net/websocket"
	"net/http"
	"strings"
	"sync"
)

type TokenHandler func(r *http.Request) (addr string, err error)

type Config struct {
	LogLevel uint32
	TokenHandler
}

type Proxy struct {
	logLevel     uint32
	peers        map[*peer]struct{}
	l            sync.RWMutex
	tokenHandler TokenHandler
}

func New(conf *Config) *Proxy {
	if conf.TokenHandler == nil {
		conf.TokenHandler = func(r *http.Request) (addr string, err error) {
			return ":5901", nil
		}
	}

	return &Proxy{
		logLevel:     conf.LogLevel,
		peers:        make(map[*peer]struct{}),
		l:            sync.RWMutex{},
		tokenHandler: conf.TokenHandler,
	}
}

func checkToken(token string) bool {
	return true
}

func (p *Proxy) ServeWS(ws *websocket.Conn) {
	log.Debugf("ServeWS")
	ws.PayloadType = websocket.BinaryFrame

	r := ws.Request()
	log.Debugf("request url: %v", r.URL)

	// get vnc backend server addr
	addr, err := p.tokenHandler(r)
	if err != nil {
		log.Infof("get vnc backend failed: %v", err)
		return
	}

	peer, err := NewPeer(ws, addr)
	if err != nil {
		log.Infof("new vnc peer failed: %v", err)
		return
	}

	p.addPeer(peer)
	defer func() {
		log.Info("close peer")
		p.deletePeer(peer)

	}()

	go func() {
		if err := peer.ReadTarget(); err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			log.Info(err)
			return
		}
	}()

	if err = peer.ReadSource(); err != nil {
		if strings.Contains(err.Error(), "use of closed network connection") {
			return
		}
		log.Info(err)
		return
	}
}

func (p *Proxy) addPeer(peer *peer) {
	p.l.Lock()
	p.peers[peer] = struct{}{}
	p.l.Unlock()
}

func (p *Proxy) deletePeer(peer *peer) {
	p.l.Lock()
	delete(p.peers, peer)
	peer.Close()
	p.l.Unlock()
}

func (p *Proxy) Peers() map[*peer]struct{} {
	return p.peers
}
