package service

import (
	"../common"
	"../component/client"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
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
	CountError  int
	lastMessage string

	TestConnectCode string
}

func (client *TestClient) ResetError() {
	client.CountError = 0
}

func (client *TestClient) Error(message string) {
	client.CountError++
	client.lastMessage = message
}

func (client *TestClient) Check() bool {
	if client.CountError > 0 {
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

func init() {
	//common.Options.DebugFlag = false
	//common.Options.Mode = common.ModeMaster
}

func TestVersionProcessing(t *testing.T) {
	c := client.Client{Serial: common.RandomString(common.CodeLength), Pass: "12345", Version: "1.0"}

	//--------------

	processVersion(createMessage(TMESS_VERSION, "2.0"), nil, &c, "TEST")
	require.True(t, c.Version == "2.0")

	processVersion(createMessage(TMESS_VERSION, "3.0", "123"), nil, &c, "TEST") //wrong arg count
	require.True(t, c.Version == "2.0")

	//--------------

	c.Version = "0.0"
	var testClient net.Conn = &TestClient{}
	require.True(t, testClient.SetDeadline(time.Now()) == nil)
	require.True(t, testClient.SetReadDeadline(time.Now()) == nil)
	require.True(t, testClient.SetWriteDeadline(time.Now()) == nil)
	require.True(t, testClient.Close() == nil)
	a, b := testClient.Read([]byte{})
	require.True(t, a == 0 && b == nil)
	testClient.(*TestClient).Error("test client")
	require.True(t, testClient.(*TestClient).Check() == false)
	require.True(t, testClient.LocalAddr().String() != testClient.RemoteAddr().String())
	require.True(t, testClient.LocalAddr().Network() != testClient.RemoteAddr().Network())

	processAuth(createMessage(TMESS_AUTH, "0"), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	serial := common.RandomString(common.LengthSalt)
	pid := common.GetPid(serial)

	processAuth(createMessage(TMESS_AUTH, serial), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check())

	processNotification(createMessage(TMESS_NOTIFICATION, "test notify"), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	processNotification(createMessage(TMESS_NOTIFICATION, pid, "test notify"), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check())

	processConnect(createMessage(TMESS_REQUEST, ""), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	processConnect(createMessage(TMESS_REQUEST, "000:000:000", "salt", "digest", "address"), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	processConnect(createMessage(TMESS_REQUEST, pid, "salt", "digest", "address"), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check())

	processPing(createMessage(TMESS_PING), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check())

	processDisconnect(createMessage(TMESS_DISCONNECT, ""), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	processConnect(createMessage(TMESS_DISCONNECT, "000:000:000"), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	processConnect(createMessage(TMESS_DISCONNECT, testClient.(*TestClient).TestConnectCode, "0"), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error
}
