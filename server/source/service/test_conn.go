package service

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type TestAddr struct {
	local bool
}

func (t TestAddr) Network() string {
	if t.local {
		return "tcp-test-local"
	}
	return "tcp-test-remote"
}

func (t TestAddr) String() string {
	if t.local {
		return "127.0.0.1:1234"
	}
	return "88.77.66.55:5432"
}

type TestClient struct {
	countError  int
	lastMessage string

	TestConnectCode string
}

func (client TestClient) ResetError() {
	client.countError = 0
}

func (client TestClient) Error(message string) {
	client.countError++
	client.lastMessage = message
}

func (client TestClient) Check() bool {
	if client.countError > 0 {
		fmt.Println("client with error: " + client.lastMessage)
		client.ResetError()
		return false
	}
	return true
}

func (TestClient) Read(b []byte) (n int, err error) {
	return len(b), nil
}

func (client TestClient) Write(b []byte) (n int, err error) {
	fmt.Println("test client got: " + string(b))

	var message Message
	err = json.Unmarshal(b, &message)
	if err != nil {
		client.Error(err.Error())
		return len(b), err
	}

	if message.TMessage == TMESS_AUTH {
		fmt.Println("client got auth message")
		if len(message.Messages) != 3 {
			client.Error("wrong count of poles")
		}
	} else if message.TMessage == TMESS_NOTIFICATION {
		fmt.Println("client got notify message")
		if len(message.Messages) != 1 {
			client.Error("wrong count of poles")
		}
	} else if message.TMessage == TMESS_PING {
		fmt.Println("client got ping message")
	} else if message.TMessage == TMESS_CONNECT {
		fmt.Println("client got connect message")
		if len(message.Messages) != 7 {
			client.Error("wrong count of poles")
			return len(b), nil
		}
		client.TestConnectCode = message.Messages[2]
	} else {
		client.Error("client got unknown message")
	}

	return len(b), nil
}

func (TestClient) Close() error {
	return nil
}

func (TestClient) LocalAddr() net.Addr {
	return TestAddr{local: true}
}

func (TestClient) RemoteAddr() net.Addr {
	return TestAddr{local: false}
}

func (TestClient) SetDeadline(t time.Time) error {
	return nil
}

func (TestClient) SetReadDeadline(t time.Time) error {
	return nil
}

func (TestClient) SetWriteDeadline(t time.Time) error {
	return nil
}
