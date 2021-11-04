package ssh

import (
	"time"

	"golang.org/x/crypto/ssh"
)

type AuthModel int8

const (
	PASSWORD AuthModel = iota + 1
	PUBLICKEY
)

type SSHClientConfig struct {
	AuthModel  AuthModel
	HostAddr   string
	User       string
	Password   string
	PrivateKey string
	Timeout    time.Duration
}

func NewSSHClient(conf *SSHClientConfig) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		Timeout:         conf.Timeout,
		User:            conf.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //忽略know_hosts检查
	}
	switch conf.AuthModel {
	case PASSWORD:
		config.Auth = []ssh.AuthMethod{ssh.Password(conf.Password)}
	case PUBLICKEY:
		signer, err := getKey(conf.PrivateKey)
		if err != nil {
			return nil, err
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}
	c, err := ssh.Dial("tcp", conf.HostAddr, config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func getKey(privateKey string) (ssh.Signer, error) {
	return ssh.ParsePrivateKey([]byte(privateKey))
}
