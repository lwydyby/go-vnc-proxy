package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	g_websocket "github.com/gorilla/websocket"
	"github.com/lwydyby/go-vnc-proxy/conf"
	"github.com/lwydyby/go-vnc-proxy/proxy"
	"github.com/lwydyby/go-vnc-proxy/ssh"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"time"
)

var logLevel uint32

func init() {
	filename, _ := filepath.Abs("./example/etc/app.yml")
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	var c conf.AppConf
	err = yaml.Unmarshal(yamlFile, &c)
	conf.SetAppConf(c)
	if err != nil {
		panic(err)
	}
	getLogLevel(conf.Conf.AppInfo.Level)
	log.SetReportCaller(true)
	log.SetFormatter(&LogFormatter{})
}

func main() {
	http.HandleFunc("/ws", proxyHandler)
	http.HandleFunc("/ssh", sshHandler)
	log.Info("vnc proxy start success ^ - ^  websocket port: " + strconv.Itoa(conf.Conf.AppInfo.Port))
	if err := http.ListenAndServe(":"+strconv.Itoa(conf.Conf.AppInfo.Port), nil); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}

//日志自定义格式
type LogFormatter struct{}

//格式详情
func (s *LogFormatter) Format(entry *log.Entry) ([]byte, error) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	var file string
	var line int
	if entry.Caller != nil {
		file = filepath.Base(entry.Caller.File)
		line = entry.Caller.Line
	}
	level := entry.Level.String()
	if entry.Context == nil || entry.Context.Value("trace_id") == "" {
		uuid, _ := GenerateUUID()
		entry.Context = context.WithValue(context.Background(), "trace_id", uuid)
	}
	msg := fmt.Sprintf("%-15s [%-3d] [%-5s] [%s] %s:%d %s\n", timestamp, getGID(), level, entry.Context.Value("trace_id"), file, line, entry.Message)
	return []byte(msg), nil
}

// 获取当前协程id
func getGID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

func NewVNCProxy() *proxy.Proxy {
	return proxy.New(&proxy.Config{
		LogLevel: logLevel,
		TokenHandler: func(r *http.Request) (addr string, err error) {
			defer func() {
				// 处理所有异常，防止panic导致程序关闭
				if p := recover(); p != nil {
					debug.PrintStack()
				}
			}()
			//todo 获取服务地址的方法
			addr = "127.0.0.1:5900"
			return
		},
	})
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	uuid, _ := GenerateUUID()
	hook := proxy.AddTraceIdHook(uuid)
	defer proxy.RemoveTraceHook(hook)
	vncProxy := NewVNCProxy()
	h := websocket.Handler(vncProxy.ServeWS)
	h.ServeHTTP(w, r)
}

func sshHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := g_websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024 * 10,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.Write([]byte("websocket create failed"))
		return
	}
	defer wsConn.Close()
	var config *ssh.SSHClientConfig
	authModel, err := getQuery(r, "auth_model")
	if err != nil {
		wsConn.WriteControl(g_websocket.CloseMessage,
			[]byte(err.Error()), time.Now().Add(time.Second))
		return
	}
	user, err := getQuery(r, "user")
	if err != nil {
		wsConn.WriteControl(g_websocket.CloseMessage,
			[]byte(err.Error()), time.Now().Add(time.Second))
		return
	}
	addr, err := getQuery(r, "addr")
	if err != nil {
		wsConn.WriteControl(g_websocket.CloseMessage,
			[]byte(err.Error()), time.Now().Add(time.Second))
		return
	}
	port, err := getQuery(r, "port")
	if err != nil {
		port = "22"
	}
	switch authModel {
	case "password":
		pwd, err := getQuery(r, "pwd")
		if err != nil {
			wsConn.WriteControl(g_websocket.CloseMessage,
				[]byte(err.Error()), time.Now().Add(time.Second))
			return
		}
		config = &ssh.SSHClientConfig{
			AuthModel: ssh.PASSWORD,
			HostAddr:  addr + ":" + port,
			User:      user,
			Password:  pwd,
			Timeout:   5 * time.Second,
		}
	case "key":
		//路径直接传输私钥有问题 暂不支持
		//todo 先提供上传私钥接口 返回文件ID 连接时携带这个ID进行匹配
		w.Write([]byte("failed"))
		return
	}
	client, err := ssh.NewSSHClient(config)
	if err != nil {
		wsConn.WriteControl(g_websocket.CloseMessage,
			[]byte(err.Error()), time.Now().Add(time.Second))
		return
	}
	defer client.Close()
	turn, err := ssh.NewTurn(wsConn, client)

	if err != nil {
		wsConn.WriteControl(g_websocket.CloseMessage,
			[]byte(err.Error()), time.Now().Add(time.Second))
		return
	}
	defer turn.Close()
	var logBuff = ssh.BufPool.Get().(*bytes.Buffer)
	logBuff.Reset()
	defer ssh.BufPool.Put(logBuff)
	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := turn.LoopRead(logBuff, ctx)
		if err != nil {
			log.Printf("%#v", err)
		}
	}()
	go func() {
		defer wg.Done()
		err := turn.SessionWait()
		if err != nil {
			log.Printf("%#v", err)
		}
		cancel()
	}()
	wg.Wait()
}

func getQuery(r *http.Request, key string) (string, error) {
	keys, ok := r.URL.Query()[key]
	if !ok || len(keys[0]) < 1 {
		return "", errors.New("Url Param 'token' is missing ")
	}
	return keys[0], nil
}

func getLogLevel(level string) {
	switch level {
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	default:
		log.SetLevel(log.DebugLevel)
	}
}
