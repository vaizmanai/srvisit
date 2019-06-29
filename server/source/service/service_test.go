package service

import (
	"../common"
	"../component/client"
	"../component/profile"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"net"
	"strings"
	"sync"
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
	mutex       sync.RWMutex
	//-----
	TestConnectCode string

	AuthSuccess         bool
	PingSuccess         bool
	RegSuccess          bool
	NotificationSuccess bool
	DeAuthSuccess       bool
	ReqSuccess          bool
	ConnectSuccess      bool
	DisconnectSuccess   bool
	LoginSuccess        bool
	ContactsSuccess     bool
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

func (client *TestClient) Write(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}

	fmt.Println("test client got: " + string(b))

	var message Message
	err = json.Unmarshal(b, &message)
	if err != nil {
		fmt.Println("message: " + string(b))
		client.Error(err.Error())
		return len(b), err
	}
	fmt.Println(message.TMessage)
	if message.TMessage == TMESS_AUTH {
		fmt.Println("client got auth message")
		if len(message.Messages) != 3 {
			client.Error("wrong count of poles")
		}
		client.AuthSuccess = true
	} else if message.TMessage == TMESS_NOTIFICATION {
		fmt.Println("client got notify message")
		if len(message.Messages) != 1 {
			client.Error("wrong count of poles")
		}
		client.NotificationSuccess = true
	} else if message.TMessage == TMESS_PING {
		fmt.Println("client got ping message")
		client.PingSuccess = true
	} else if message.TMessage == TMESS_CONNECT {
		fmt.Println("client got connect message")
		if len(message.Messages) != 7 {
			client.Error("wrong count of poles")
			return len(b), nil
		}
		client.TestConnectCode = message.Messages[2]
		client.ReqSuccess = true
	} else if message.TMessage == TMESS_REG {
		if len(message.Messages) != 1 {
			client.Error("wrong count of poles")
			return len(b), nil
		}
		if message.Messages[0] == "success" {
			client.RegSuccess = true
		} else {
			client.RegSuccess = false
		}
	} else if message.TMessage == TMESS_LOGIN {
		if len(message.Messages) != 0 {
			client.Error("wrong count of poles")
			return len(b), nil
		}
		client.LoginSuccess = true
	} else if message.TMessage == TMESS_CONTACTS {
		client.ContactsSuccess = true
	} else if message.TMessage == TMESS_CONTACT {
		//client.ContactsSuccess = true
	} else if message.TMessage == TMESS_STATUS {
		//client.ContactsSuccess = true
	} else if message.TMessage == TMESS_DEAUTH {
		client.DeAuthSuccess = true
	} else {
		client.Error("client got unknown message: " + fmt.Sprint(message.TMessage))
	}

	return len(b), nil
}

func (TestClient) Close() error {
	return nil
}

func (client *TestClient) LocalAddr() net.Addr {
	client.mutex.RLock()
	defer client.mutex.RUnlock()
	return TestAddr{local: true}
}

func (client *TestClient) RemoteAddr() net.Addr {
	client.mutex.RLock()
	defer client.mutex.RUnlock()
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

func TestStaticProcessing(t *testing.T) {
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
	c.Conn = &testClient

	require.True(t, a == 0 && b == nil)
	testClient.(*TestClient).Error("test client")
	require.True(t, testClient.(*TestClient).Check() == false)
	require.True(t, testClient.LocalAddr().String() != testClient.RemoteAddr().String())
	require.True(t, testClient.LocalAddr().Network() != testClient.RemoteAddr().Network())

	processAuth(createMessage(TMESS_AUTH), &testClient, &c, "TEST1")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	processAuth(createMessage(TMESS_AUTH, "0"), &testClient, &c, "TEST1")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error
	require.True(t, testClient.(*TestClient).DeAuthSuccess == true)

	serial := common.RandomString(common.LengthSalt)
	pid := common.GetPid(serial)

	processAuth(createMessage(TMESS_AUTH, serial), &testClient, &c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).AuthSuccess == true)

	processNotification(createMessage(TMESS_NOTIFICATION, "test notify"), &testClient, &c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).NotificationSuccess == false)

	processNotification(createMessage(TMESS_NOTIFICATION, pid, "test notify"), &testClient, &c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).NotificationSuccess == true)

	processConnect(createMessage(TMESS_REQUEST, ""), &testClient, &c, "TEST1")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error
	require.True(t, testClient.(*TestClient).ReqSuccess == false)

	processConnect(createMessage(TMESS_REQUEST, "000:000:000", "salt", "digest", "address"), &testClient, &c, "TEST2")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error
	require.True(t, testClient.(*TestClient).ReqSuccess == false)

	processConnect(createMessage(TMESS_REQUEST, pid, "salt", "digest", "address"), &testClient, &c, "TEST3")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).ReqSuccess == true)

	processPing(createMessage(TMESS_PING), &testClient, &c, "TEST") //сервер ничего не отвечает на пинг
	require.True(t, testClient.(*TestClient).Check())

	processDisconnect(createMessage(TMESS_DISCONNECT, ""), &testClient, &c, "TEST1")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	processConnect(createMessage(TMESS_DISCONNECT, "000:000:000"), &testClient, &c, "TEST2")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	processConnect(createMessage(TMESS_DISCONNECT, testClient.(*TestClient).TestConnectCode, "0"), &testClient, &c, "TEST3")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	processReg(createMessage(TMESS_REG), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).RegSuccess == false)

	email := strings.ToLower(common.RandomString(common.LengthSalt) + "@mail.net")
	processReg(createMessage(TMESS_REG, email), &testClient, &c, "TEST")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).RegSuccess == true)
	p := profile.GetProfile(email)
	require.True(t, p != nil)
	require.True(t, p.Pass == common.PredefinedPass)

	processLogin(createMessage(TMESS_LOGIN), &testClient, &c, "TEST1")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error
	require.True(t, testClient.(*TestClient).LoginSuccess == false)

	processLogin(createMessage(TMESS_LOGIN, "root@mail.net", "password"), &testClient, &c, "TEST2")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error
	require.True(t, testClient.(*TestClient).LoginSuccess == false)

	processLogin(createMessage(TMESS_LOGIN, email, common.GetSHA256(common.PredefinedPass+c.Salt)), &testClient, &c, "TEST3")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error
	require.True(t, testClient.(*TestClient).LoginSuccess == true)
	require.True(t, testClient.(*TestClient).ContactsSuccess == true)
	require.True(t, len(client.GetAuthorizedClientList(email)) == 1)

	processLogout(createMessage(TMESS_LOGOUT, email, common.GetSHA256(common.PredefinedPass+c.Salt)), &testClient, &c, "TEST1")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error
	require.True(t, len(client.GetAuthorizedClientList(email)) == 0)

	processLogout(createMessage(TMESS_LOGOUT, email, common.GetSHA256(common.PredefinedPass+c.Salt)), &testClient, &c, "TEST2")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error
	require.True(t, len(client.GetAuthorizedClientList(email)) == 0)

	//--------
	fmt.Println("!!!!!!!!!!!!!!!!!!!!")

	processContact(createMessage(TMESS_CONTACT), &testClient, &c, "TEST1")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	processContact(createMessage(TMESS_CONTACT, "1", "2", "3", "4", "5", "6"), &testClient, &c, "TEST2")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	processLogin(createMessage(TMESS_LOGIN, email, common.GetSHA256(common.PredefinedPass+c.Salt)), &testClient, &c, "TEST3")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	processContact(createMessage(TMESS_CONTACT, "a123", "2", "3", "4", "5", "6"), &testClient, &c, "TEST4")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

	//1 - id
	//2 - del/type
	//3 - caption
	//4 - pid
	//5 - digest
	//6 - parent(not necessary)
	processContact(createMessage(TMESS_CONTACT, "1", "2", "3", "4", "5", "6"), &testClient, &c, "TEST5")
	require.True(t, testClient.(*TestClient).Check()) //todo переделать на проверку возврата error

}

func creationClient() bool {
	serial := common.RandomString(common.LengthSalt)

	time.Sleep(time.Duration(common.RandInt(0, 5)) * time.Second)

	conn, err := net.Dial("tcp", "127.0.0.1:"+common.Options.MainServerPort)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}

	if !sendMessage(&conn, TMESS_AUTH, serial) {
		return false
	}

	//todo read response

	time.Sleep(time.Duration(common.RandInt(0, 10)) * time.Second)

	return true
}

//func TestThreadClient(t *testing.T) {
//
//	countThread := 100
//	done := make(chan bool)
//
//	go MainServer()
//
//	fail := false
//	var mutex sync.Mutex
//
//	for i := 0; i < countThread; i++ {
//
//		go func(n int) {
//			r := creationClient()
//			if !r {
//				mutex.Lock()
//				fail = true
//				mutex.Unlock()
//			}
//			done <- true
//		}(i)
//
//	}
//
//	for i := 0; i < countThread; i++ {
//		<-done
//	}
//
//	require.True(t, fail == false)
//}
