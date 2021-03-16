package proxy

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/lwydyby/go-vnc-proxy/conf"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

const (
	VERSION_LENGTH   = 12
	SUBTYPE_LENGTH   = 4
	AUTH_STSTUS_FAIL = "\x00"
	AUTH_STATUS_PASS = "\x01"
	PVLEN            = 12
)

type AuthType = int

var (
	INVALID  AuthType = 0
	NONE     AuthType = 1
	VNC      AuthType = 2
	VENCRYPT AuthType = 19
	X509NONE          = 260
)

func Connect(addr string, source net.Conn, target net.Conn) (net.Conn, error) {
	isVencrypt, err := checkIsVencrypt(addr)
	if err != nil {
		return nil, err
	}
	if !isVencrypt {
		return target, nil
	}
	serverName := strings.Split(addr, ":")[0]
	targetVersion, err := recv(target, VERSION_LENGTH)
	if err != nil {
		return nil, err
	}
	tv := parseVersion(targetVersion)
	if tv != 3.8 {
		return nil, errors.New("Security proxying requires RFB protocol version 3.8 , but tenant asked for " + string(targetVersion))
	}
	_, err = target.Write(targetVersion)
	if err != nil {
		return nil, err
	}
	_, err = source.Write(targetVersion)
	if err != nil {
		return nil, err
	}
	sourceVersion, err := recv(source, VERSION_LENGTH)
	if err != nil {
		return nil, err
	}
	v := parseVersion(sourceVersion)
	if v != 3.8 {
		return nil, errors.New("Security proxying requires RFB protocol version 3.8 , but tenant asked for " + string(sourceVersion))
	}

	authType, err := recv(target, 1)
	if err != nil {
		return nil, err
	}
	if byte2int(authType) == 0 {
		return nil, errors.New("negotiation failed: " + string(authType))
	}
	f, err := recv(target, byte2int(authType))
	if err != nil {
		return nil, err
	}
	permittedAuthType := make([]int, 0)
	for _, t := range f {
		permittedAuthType = append(permittedAuthType, int(t))
	}
	data := []byte("\x01\x01")
	source.Write(data)
	clientAuth, _ := recv(source, 1)
	if byte2int(clientAuth) != NONE {
		return nil, errors.New("negotiation failed: " + string(clientAuth))
	}
	if permittedAuthType[0] != VENCRYPT {
		log.Warn("is not VENCRYPT conn")
		return nil, errors.New("s not VENCRYPT conn")
	}
	target.Write(f)
	return SecurityHandshake(serverName, target)
}

func checkIsVencrypt(addr string) (bool, error) {
	target, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return false, err
	}
	targetVersion, err := recv(target, VERSION_LENGTH)
	if err != nil {
		return false, err
	}
	tv := parseVersion(targetVersion)
	if tv != 3.8 {
		return false, errors.New("Security proxying requires RFB protocol version 3.8 , but tenant asked for " + string(targetVersion))
	}
	_, err = target.Write(targetVersion)
	if err != nil {
		return false, err
	}
	authType, err := recv(target, 1)
	if err != nil {
		return false, err
	}
	if byte2int(authType) == 0 {
		return false, errors.New("negotiation failed: " + string(authType))
	}
	f, err := recv(target, byte2int(authType))
	if err != nil {
		return false, err
	}
	permittedAuthType := make([]int, 0)
	for _, t := range f {
		permittedAuthType = append(permittedAuthType, int(t))
	}
	err = target.Close()
	if err != nil {
		return false, err
	}
	return permittedAuthType[0] == VENCRYPT, nil
}

func SecurityHandshake(serverName string, target net.Conn) (net.Conn, error) {
	maj, _ := recv(target, 1)
	min, _ := recv(target, 1)
	majVer := byte2int(maj)
	minVer := byte2int(min)
	log.Debugf("Server sent VeNCrypt version %v.%v", majVer, minVer)
	if majVer != 0 || minVer != 2 {
		return nil, errors.New(fmt.Sprintf("Only VeNCrypt version 0.2 is supported by this proxy, but the server wanted to use version :%v.%v", majVer, minVer))
	}
	data := [2]byte{'\x00', '\x02'}
	err := send(target, data)
	if err != nil {
		return nil, err
	}
	var isAccepted uint8
	err = receive(target, &isAccepted)
	if err != nil {
		return nil, err
	}
	if isAccepted > 0 {
		return nil, errors.New("Server could not use VeNCrypt version 0.2 ")
	}
	subTypesCnt, _ := recv(target, 1)
	subAuthTypes := make([]int32, byte2int(subTypesCnt))
	err = receiveN(target, &subAuthTypes, byte2int(subTypesCnt))
	if err != nil {
		return nil, err
	}
	hasX509 := false
	for _, t := range subAuthTypes {
		if t == int32(X509NONE) {
			hasX509 = true
			break
		}
	}
	if !hasX509 {
		return nil, errors.New("Server does not support the x509None VeNCrypt ")
	}
	send(target, uint32(X509NONE))
	authAccepted, _ := recv(target, 1)
	if byte2int(authAccepted) == 0 {
		return nil, errors.New("Server didn't accept the requested auth sub-type ")
	}
	config := &tls.Config{
		InsecureSkipVerify: true,
	}
	//fixme 对于没有使用双向加密的vnc,可能不需要证书
	if conf.Conf.AppInfo.TLSCert != "" {
		cert, err := tls.LoadX509KeyPair(conf.Conf.AppInfo.TLSCert, conf.Conf.AppInfo.TLSKey)
		if err != nil {
			panic(err)
		}
		certBytes, err := ioutil.ReadFile(conf.Conf.AppInfo.TLSCaCerts)
		if err != nil {
			panic(err)
		}
		clientCertPool := x509.NewCertPool()
		ok := clientCertPool.AppendCertsFromPEM(certBytes)
		if !ok {
			panic(err)
		}
		config.RootCAs = clientCertPool
		config.Certificates = []tls.Certificate{cert}
	}
	conn := tls.Client(target, config)
	return conn, nil
}

func receiveN(c net.Conn, data interface{}, n int) error {
	if n == 0 {
		return nil
	}

	switch data.(type) {
	case *[]uint8:
		var v uint8
		for i := 0; i < n; i++ {
			if err := binary.Read(c, binary.BigEndian, &v); err != nil {
				return err
			}
			slice := data.(*[]uint8)
			*slice = append(*slice, v)
		}
	case *[]int32:
		var v int32
		for i := 0; i < n; i++ {
			if err := binary.Read(c, binary.BigEndian, &v); err != nil {
				return err
			}
			slice := data.(*[]int32)
			*slice = append(*slice, v)
		}
	case *bytes.Buffer:
		var v byte
		for i := 0; i < n; i++ {
			if err := binary.Read(c, binary.BigEndian, &v); err != nil {
				return err
			}
			buf := data.(*bytes.Buffer)
			buf.WriteByte(v)
		}
	default:
		return errors.New(fmt.Sprintf("unrecognized data type %v", reflect.TypeOf(data)))
	}
	return nil
}

func receive(c net.Conn, data interface{}) error {
	if err := binary.Read(c, binary.BigEndian, data); err != nil {
		return err
	}
	return nil
}

// send a packet to the network.
func send(c net.Conn, data interface{}) error {
	if err := binary.Write(c, binary.BigEndian, data); err != nil {
		return err
	}
	return nil
}

func IntToBytes(data int) (ret []byte) {
	var len uintptr = unsafe.Sizeof(data)
	ret = make([]byte, len)
	var tmp int = 0xff
	var index uint = 0
	for index = 0; index < uint(len); index++ {
		ret[index] = byte((tmp << (index * 8) & data) >> (index * 8))
	}
	return ret
}

func byte2int(data []byte) int {
	var ret int = 0
	var len int = len(data)
	var i uint = 0
	for i = 0; i < uint(len); i++ {
		ret = ret | (int(data[i]) << (i * 8))
	}
	return ret
}

func parseVersion(version []byte) float64 {
	versionStr := string(version)

	result, _ := strconv.ParseFloat(fmt.Sprintf("%v.%v", str2int(versionStr[4:7]), str2int(versionStr[8:11])), 1)
	return result
}
func str2int(s string) int {
	result, _ := strconv.Atoi(s)
	return result
}

func recv(c net.Conn, num int) ([]byte, error) {
	buf := make([]byte, num)
	length, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	if length != num {
		log.Warnf("Incorrect read from socket, wanted %v bytes but got %v. Socket returned", num, length)
	}
	return buf, nil
}

//BytesCombine 多个[]byte数组合并成一个[]byte
func BytesCombine(pBytes ...[]byte) []byte {
	len := len(pBytes)
	s := make([][]byte, len)
	for index := 0; index < len; index++ {
		s[index] = pBytes[index]
	}
	sep := []byte("")
	return bytes.Join(s, sep)
}
