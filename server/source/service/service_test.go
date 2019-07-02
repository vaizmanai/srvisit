package service

import (
	"../common"
	"../component/client"
	"../component/profile"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net"
	"net/http"
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
	countError   int
	lastMessages []string
	lastCode     int
	mutex        sync.RWMutex
	//-----
	TestConnectCode string
	TestContactId   string

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

func (client *TestClient) ResetFlags() {
	client.AuthSuccess = false
	client.PingSuccess = false
	client.RegSuccess = false
	client.NotificationSuccess = false
	client.DeAuthSuccess = false
	client.ReqSuccess = false
	client.ConnectSuccess = false
	client.DisconnectSuccess = false
	client.LoginSuccess = false
	client.ContactsSuccess = false
}

func (client *TestClient) Last() (int, []string) {
	code := client.lastCode
	client.lastCode = -1
	return code, client.lastMessages
}

func (client *TestClient) ResetError() {
	client.countError = 0
}

func (client *TestClient) Error(message string) {
	client.countError++
	client.lastMessages = make([]string, 1)
	client.lastMessages[0] = message
}

func (client *TestClient) Check() bool {
	if client.countError > 0 {
		fmt.Println("client with error: " + client.lastMessages[0])
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

	client.lastCode = message.TMessage
	client.lastMessages = message.Messages
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
	} else if message.TMessage == TMESS_STANDART_ALERT {
		//client.ContactsSuccess = true
	} else if message.TMessage == TMESS_CONTACT {
		//client.ContactsSuccess = true
		client.TestContactId = message.Messages[0]
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
	c := &client.Client{Serial: common.RandomString(common.CodeLength), Pass: "12345", Version: "1.0"}

	//--------------

	//успешный
	r := processVersion(createMessage(TMESS_VERSION, "2.0"), nil, c, "TEST")
	require.True(t, c.Version == "2.0")
	require.True(t, r == true)

	//не правильное кол-во полей
	r = processVersion(createMessage(TMESS_VERSION, "3.0", "123"), nil, c, "TEST") //wrong arg count
	require.True(t, c.Version == "2.0")
	require.True(t, r == false)

	//--------------

	//проверяем что тестовый клиент работает
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

	//не правильное кол-во полей
	r = processAuth(createMessage(TMESS_AUTH), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//слабый serial
	r = processAuth(createMessage(TMESS_AUTH, "0"), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).DeAuthSuccess == true)
	require.True(t, r == false)

	serial := common.RandomString(common.LengthSalt)
	pid := common.GetPid(serial)

	//успешный
	r = processAuth(createMessage(TMESS_AUTH, serial), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).AuthSuccess == true)
	code, mess := testClient.(*TestClient).Last()
	require.True(t, r == true)
	require.True(t, code == TMESS_AUTH && mess[0] == pid)

	//не правильное кол-во полей
	r = processNotification(createMessage(TMESS_NOTIFICATION, "test notify"), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).NotificationSuccess == false)
	require.True(t, r == false)

	//успешный
	r = processNotification(createMessage(TMESS_NOTIFICATION, pid, "test notify"), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).NotificationSuccess == true)
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	require.True(t, code == TMESS_NOTIFICATION && mess[0] == "test notify")

	//не правильное кол-во полей
	r = processConnect(createMessage(TMESS_REQUEST, ""), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).ReqSuccess == false)
	require.True(t, r == false)

	//нет такого пира
	r = processConnect(createMessage(TMESS_REQUEST, "000:000:000", "salt", "digest", "address"), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).ReqSuccess == false)
	require.True(t, r == false)
	code, mess = testClient.(*TestClient).Last()
	require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Нет такого пира")

	//успешный
	r = processConnect(createMessage(TMESS_REQUEST, pid, "salt", "digest", "address"), &testClient, c, "TEST3")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).ReqSuccess == true)
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	require.True(t, code == TMESS_CONNECT && mess[0] == "salt" && mess[1] == "digest" && mess[5] == pid)

	//сервер ничего не отвечает на пинг
	r = processPing(createMessage(TMESS_PING), &testClient, c, "TEST")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)

	//не правильное кол-во полей
	r = processDisconnect(createMessage(TMESS_DISCONNECT), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//пустой ид
	r = processDisconnect(createMessage(TMESS_DISCONNECT, ""), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//пробует отключить, то что нет такого соединения не считаем ошибкой и никому ничего не шлем
	r = processDisconnect(createMessage(TMESS_DISCONNECT, "000:000:000"), &testClient, c, "TEST3")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)

	r = processDisconnect(createMessage(TMESS_DISCONNECT, testClient.(*TestClient).TestConnectCode, "0"), &testClient, c, "TEST4")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)

	//не правильное кол-во полей
	r = processReg(createMessage(TMESS_REG), &testClient, c, "TEST")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).RegSuccess == false)
	require.True(t, r == false)

	//успешный
	email := strings.ToLower(common.RandomString(common.LengthSalt) + "@mail.net")
	r = processReg(createMessage(TMESS_REG, email), &testClient, c, "TEST")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).RegSuccess == true)
	p := profile.GetProfile(email)
	require.True(t, p != nil)
	require.True(t, p.Pass == common.PredefinedPass)
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Учетная запись создана, Ваш пароль на почте!")

	//учетка занята
	r = processReg(createMessage(TMESS_REG, email), &testClient, c, "TEST")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).RegSuccess == true)
	code, mess = testClient.(*TestClient).Last()
	require.True(t, r == true)
	if c.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		require.True(t, code == TMESS_STANDART_ALERT && mess[0] == fmt.Sprint(common.StaticMessageRegFail))
	} else {
		require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Такая учетная запись уже существует!")
	}

	c.Version = "0.4"
	testProfile(t, testClient, c, email)

	c.Version = "1.3"
	testProfile(t, testClient, c, email)

	fmt.Println("---------------------------------------------")

	testThreadClient(t)
	testWebThreads(t)
}

func testProfile(t *testing.T, testClient net.Conn, c *client.Client, email string) {
	testClient.(*TestClient).ResetFlags()
	profile.GetProfile(email).Contacts = nil

	//успешный
	common.Options.ServerSMTP = "smtp.gmail.com"
	email1 := strings.ToLower(common.RandomString(common.LengthSalt) + "@mail.net")
	r := processReg(createMessage(TMESS_REG, email1), &testClient, c, "TEST")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).RegSuccess == false)
	code, mess := testClient.(*TestClient).Last()
	if c.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		require.True(t, code == TMESS_STANDART_ALERT && mess[0] == fmt.Sprint(common.StaticMessageRegMail))
	} else {
		require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Не удалось отправить письмо с паролем!")
	}
	require.True(t, profile.GetProfile(email1) == nil)

	time.Sleep(time.Second)

	//не правильное кол-во полей
	r = processLogin(createMessage(TMESS_LOGIN), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).LoginSuccess == false)
	require.True(t, r == false)

	r = processLogin(createMessage(TMESS_LOGIN, "root@mail.net", "password"), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).LoginSuccess == false)
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	if c.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		require.True(t, code == TMESS_STANDART_ALERT && mess[0] == fmt.Sprint(common.StaticMessageAuthFail))
	} else {
		require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Авторизация профиля провалилась!")
	}

	r = processLogin(createMessage(TMESS_LOGIN, email, common.GetSHA256(common.PredefinedPass+c.Salt)), &testClient, c, "TEST3")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, testClient.(*TestClient).LoginSuccess == true)
	require.True(t, testClient.(*TestClient).ContactsSuccess == true)
	require.True(t, len(client.GetAuthorizedClientList(email)) == 1)
	require.True(t, r == true)
	code, _ = testClient.(*TestClient).Last()
	require.True(t, code == TMESS_CONTACTS) //шлем сначала LOGIN и сразу контакты

	r = processLogout(createMessage(TMESS_LOGOUT), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, len(client.GetAuthorizedClientList(email)) == 0)
	require.True(t, r == true)

	r = processLogout(createMessage(TMESS_LOGOUT), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, len(client.GetAuthorizedClientList(email)) == 0)
	require.True(t, r == false)

	r = processContact(createMessage(TMESS_CONTACT), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	r = processContact(createMessage(TMESS_CONTACT, "1", "2", "3", "4", "5", "6"), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	r = processLogin(createMessage(TMESS_LOGIN, email, common.GetSHA256(common.PredefinedPass+c.Salt)), &testClient, c, "TEST3")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)

	//пустой индекс
	r = processStatus(createMessage(TMESS_STATUS, ""), &testClient, c, "TEST5")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	r = processContact(createMessage(TMESS_CONTACT, "a123", "2", "3", "4", "5", "6"), &testClient, c, "TEST4")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//--------

	//создадим структуру контактов
	//- group1
	//  - cont1
	//  - cont2
	//- group2
	//  - group3
	//    - cont3
	//    - cont4
	//- group4
	//- cont5

	//processContact(createMessage(TMESS_CONTACT, "0", "1", "2", "3", "4", "5"), &testClient, c, "TEST5")
	//0 - id
	//1 - del/type
	//2 - caption
	//3 - pid
	//4 - digest
	//5 - parent(not necessary)
	r = processContact(createMessage(TMESS_CONTACT, "-1", "fold", "group1", "", "", ""), &testClient, c, "TEST5")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	group1 := testClient.(*TestClient).TestContactId

	r = processContact(createMessage(TMESS_CONTACT, "-1", "cont", "cont1", "111:111:111:111", "digest1", group1), &testClient, c, "TEST5")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)

	r = processContact(createMessage(TMESS_CONTACT, "-1", "cont", "cont2", "222:222:222:222", "digest2", group1), &testClient, c, "TEST5")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)

	r = processContact(createMessage(TMESS_CONTACT, "-1", "fold", "group2", "", "", ""), &testClient, c, "TEST5")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	group2 := testClient.(*TestClient).TestContactId

	r = processContact(createMessage(TMESS_CONTACT, "-1", "fold", "group3", "", "", group2), &testClient, c, "TEST5")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	group3 := testClient.(*TestClient).TestContactId

	r = processContact(createMessage(TMESS_CONTACT, "-1", "cont", "cont3", "333:333:333:333", "digest3", group3), &testClient, c, "TEST5")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	cont3 := testClient.(*TestClient).TestContactId

	r = processContact(createMessage(TMESS_CONTACT, "-1", "cont", "cont4", "444:444:444:444", "digest4", group3), &testClient, c, "TEST5")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	cont4 := testClient.(*TestClient).TestContactId

	r = processContact(createMessage(TMESS_CONTACT, "-1", "fold", "group4", "", "", ""), &testClient, c, "TEST5")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)

	r = processContact(createMessage(TMESS_CONTACT, "-1", "cont", "cont5", "555:555:555:555", "digest5", ""), &testClient, c, "TEST5")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)

	bytes, error := json.Marshal(*c.Profile.Contacts)
	require.True(t, error == nil)
	testContactsString1 := `{"Id":16,"Caption":"cont5","Type":"cont","Pid":"555:555:555:555","Digest":"digest5","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":{"Id":15,"Caption":"group4","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":null,"Next":{"Id":6,"Caption":"group2","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":{"Id":7,"Caption":"group3","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":{"Id":12,"Caption":"cont4","Type":"cont","Pid":"444:444:444:444","Digest":"digest4","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":{"Id":9,"Caption":"cont3","Type":"cont","Pid":"333:333:333:333","Digest":"digest3","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":null}},"Next":null},"Next":{"Id":1,"Caption":"group1","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":{"Id":4,"Caption":"cont2","Type":"cont","Pid":"222:222:222:222","Digest":"digest2","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":{"Id":2,"Caption":"cont1","Type":"cont","Pid":"111:111:111:111","Digest":"digest1","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":null}},"Next":null}}}}`
	require.True(t, testContactsString1 == string(bytes))

	//--------

	r = processContact(createMessage(TMESS_CONTACT, cont4, "del", "", "", "", ""), &testClient, c, "TEST5")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)

	bytes, error = json.Marshal(*c.Profile.Contacts)
	require.True(t, error == nil)
	testContactsString2 := `{"Id":16,"Caption":"cont5","Type":"cont","Pid":"555:555:555:555","Digest":"digest5","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":{"Id":15,"Caption":"group4","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":null,"Next":{"Id":6,"Caption":"group2","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":{"Id":7,"Caption":"group3","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":{"Id":9,"Caption":"cont3","Type":"cont","Pid":"333:333:333:333","Digest":"digest3","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":null},"Next":null},"Next":{"Id":1,"Caption":"group1","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":{"Id":4,"Caption":"cont2","Type":"cont","Pid":"222:222:222:222","Digest":"digest2","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":{"Id":2,"Caption":"cont1","Type":"cont","Pid":"111:111:111:111","Digest":"digest1","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":null}},"Next":null}}}}`
	require.True(t, testContactsString2 == string(bytes))

	//--------

	r = processContact(createMessage(TMESS_CONTACT, cont3, "cont", "cont3moved", "333:333:333:333", "digest3", group1), &testClient, c, "TEST5")
	bytes, _ = json.Marshal(*c.Profile.Contacts)
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	testContactsString3 := `{"Id":16,"Caption":"cont5","Type":"cont","Pid":"555:555:555:555","Digest":"digest5","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":{"Id":15,"Caption":"group4","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":null,"Next":{"Id":6,"Caption":"group2","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":{"Id":7,"Caption":"group3","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":null,"Next":null},"Next":{"Id":1,"Caption":"group1","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":{"Id":9,"Caption":"cont3moved","Type":"cont","Pid":"333:333:333:333","Digest":"digest3","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":{"Id":4,"Caption":"cont2","Type":"cont","Pid":"222:222:222:222","Digest":"digest2","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":{"Id":2,"Caption":"cont1","Type":"cont","Pid":"111:111:111:111","Digest":"digest1","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":null}}},"Next":null}}}}`
	require.True(t, testContactsString3 == string(bytes))

	//--------

	r = processContact(createMessage(TMESS_CONTACT, cont3, "cont", "cont3root", "333:333:333:333", "digest3", "12345"), &testClient, c, "TEST5")
	bytes, _ = json.Marshal(*c.Profile.Contacts)
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	testContactsString5 := `{"Id":9,"Caption":"cont3root","Type":"cont","Pid":"333:333:333:333","Digest":"digest3","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":{"Id":16,"Caption":"cont5","Type":"cont","Pid":"555:555:555:555","Digest":"digest5","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":{"Id":15,"Caption":"group4","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":null,"Next":{"Id":6,"Caption":"group2","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":{"Id":7,"Caption":"group3","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":null,"Next":null},"Next":{"Id":1,"Caption":"group1","Type":"fold","Pid":"","Digest":"","Salt":"","Inner":{"Id":4,"Caption":"cont2","Type":"cont","Pid":"222:222:222:222","Digest":"digest2","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":{"Id":2,"Caption":"cont1","Type":"cont","Pid":"111:111:111:111","Digest":"digest1","Salt":"JJPJZPFRFEGMOTAF","Inner":null,"Next":null}},"Next":null}}}}}`
	require.True(t, testContactsString5 == string(bytes))

	r = processContact(createMessage(TMESS_CONTACT, cont3, "cont", "cont3root", "333:333:333:333", "digest3", "a123"), &testClient, c, "TEST5")
	bytes, _ = json.Marshal(*c.Profile.Contacts)
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	require.True(t, code == TMESS_CONTACT && fmt.Sprint(mess) == `[9 cont cont3root 333:333:333:333 digest3 a123]`)

	//--------

	//пустой индекс
	r = processStatuses(createMessage(TMESS_STATUSES, ""), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	r = processInfoContact(createMessage(TMESS_INFO_CONTACT, "9"), &testClient, c, "TEST3")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	if c.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		require.True(t, code == TMESS_STANDART_ALERT && mess[0] == fmt.Sprint(common.StaticMessageAbsentError))
	} else {
		require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Нет такого контакта в сети!")
	}

	r = processInfoContact(createMessage(TMESS_INFO_CONTACT, "-1"), &testClient, c, "TEST7")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	if c.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		require.True(t, code == TMESS_STANDART_ALERT && mess[0] == fmt.Sprint(common.StaticMessageAbsentError))
	} else {
		require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Нет такого контакта в профиле!")
	}

	r = processInfoContact(createMessage(TMESS_INFO_CONTACT, "a123"), &testClient, c, "TEST7")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	if c.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		require.True(t, code == TMESS_STANDART_ALERT && mess[0] == fmt.Sprint(common.StaticMessageAbsentError))
	} else {
		require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Ошибка преобразования идентификатора!")
	}

	//нет такого контакта в профиле
	r = processManage(createMessage(TMESS_MANAGE, "0", "2"), &testClient, c, "TEST4")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	if c.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		require.True(t, code == TMESS_STANDART_ALERT && mess[0] == fmt.Sprint(common.StaticMessageAbsentError))
	} else {
		require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Нет такого контакта в профиле!")
	}

	//ошибка индекса
	r = processManage(createMessage(TMESS_MANAGE, "a123", "2"), &testClient, c, "TEST4")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	if c.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		require.True(t, code == TMESS_STANDART_ALERT && mess[0] == fmt.Sprint(common.StaticMessageAbsentError))
	} else {
		require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Ошибка преобразования идентификатора!")
	}

	r = processConnectContact(createMessage(TMESS_CONNECT_CONTACT, "1"), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	if c.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		require.True(t, code == TMESS_STANDART_ALERT && mess[0] == fmt.Sprint(common.StaticMessageAbsentError))
	} else {
		require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Нет такого пира")
	}

	r = processConnectContact(createMessage(TMESS_CONNECT_CONTACT, "-1"), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	if c.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		require.True(t, code == TMESS_STANDART_ALERT && mess[0] == fmt.Sprint(common.StaticMessageAbsentError))
	} else {
		require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Нет такого контакта в профиле!")
	}

	r = processConnectContact(createMessage(TMESS_CONNECT_CONTACT, "a123"), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	if c.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		require.True(t, code == TMESS_STANDART_ALERT && mess[0] == fmt.Sprint(common.StaticMessageAbsentError))
	} else {
		require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Ошибка преобразования идентификатора!")
	}

	r = processContactReverse(createMessage(TMESS_CONTACT_REVERSE, email, common.GetSHA256(profile.GetProfile(email).Pass+c.Salt), "a123"), &testClient, c, "TEST7")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	require.True(t, code == TMESS_STATUS && testClient.(*TestClient).TestContactId == mess[0] && mess[1] == "1")

	r = processLogout(createMessage(TMESS_LOGOUT), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, len(client.GetAuthorizedClientList(email)) == 0)
	require.True(t, r == true)

	//--------

	//мало полей
	r = processConnectContact(createMessage(TMESS_CONNECT_CONTACT), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//не авторизованный профиль
	r = processConnectContact(createMessage(TMESS_CONNECT_CONTACT, "1"), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//--------

	//не правильное кол-во аргументов
	r = processStatuses(createMessage(TMESS_STATUSES, "1"), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//не авторизованный профиль
	r = processStatuses(createMessage(TMESS_STATUSES), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//--------

	//не правильное кол-во аргументов
	r = processStatus(createMessage(TMESS_STATUS), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//не авторизованный профиль
	r = processStatus(createMessage(TMESS_STATUS, "1"), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//пустой индекс
	//r = processStatus(createMessage(TMESS_STATUS, ""), &testClient, c, "TEST3")
	//require.True(t, testClient.(*TestClient).Check())
	//require.True(t, r == false)

	//--------

	//не авторизованный профиль
	r = processContacts(createMessage(TMESS_STATUS, "1"), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//--------

	//не правильное кол-во аргументов
	r = processInfoContact(createMessage(TMESS_INFO_CONTACT), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//не авторизованный профиль
	r = processInfoContact(createMessage(TMESS_INFO_CONTACT, "1"), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//--------

	//не правильное кол-во аргументов
	r = processManage(createMessage(TMESS_MANAGE), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//не авторизованный профиль
	r = processManage(createMessage(TMESS_MANAGE, "1", "2"), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//--------

	//не правильное кол-во аргументов
	r = processContactReverse(createMessage(TMESS_CONTACT_REVERSE), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//не авторизованный профиль
	r = processContactReverse(createMessage(TMESS_CONTACT_REVERSE, "1", "2", "3"), &testClient, c, "TEST2")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	//--------

	//не правильное кол-во аргументов
	r = processInfoAnswer(createMessage(TMESS_INFO_ANSWER), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

	r = processInfoAnswer(createMessage(TMESS_INFO_ANSWER, "111:111:222:345"), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == true)
	code, mess = testClient.(*TestClient).Last()
	if c.GreaterVersionThan(common.MinimalVersionForStaticAlert) {
		require.True(t, code == TMESS_STANDART_ALERT && mess[0] == fmt.Sprint(common.StaticMessageAbsentError))
	} else {
		require.True(t, code == TMESS_NOTIFICATION && mess[0] == "Нет такого контакта в сети!")
	}

	//--------

	//при ModeRegular у нас так и так должен возвращаться false
	r = processServers(createMessage(TMESS_SERVERS), &testClient, c, "TEST1")
	require.True(t, testClient.(*TestClient).Check())
	require.True(t, r == false)

}

func testWebThreads(t *testing.T) {
	go HttpServer()

	countThread := 1000
	done := make(chan bool)

	time.Sleep(time.Second)

	fail := false
	var mutex sync.Mutex

	for i := 0; i < countThread; i++ {

		go func() {
			r := creationWebClient()
			if !r {
				mutex.Lock()
				fail = true
				mutex.Unlock()
			}
			done <- true
		}()

	}

	for i := 0; i < countThread; i++ {
		<-done
	}

	require.True(t, fail == false)
}

func creationWebClient() bool {
	testMethods := []string{"GET", "POST", "DELETE", "PUT", "OPTIONS"}
	testNewRequests := []string{"/v2/api", "/v2/api/auth", "/v2/api/client", "/v2/api/clients", "/v2/api/profiles"}

	method := testMethods[common.RandInt(0, len(testMethods))]
	url := testNewRequests[common.RandInt(0, len(testNewRequests))]
	desc := method + " " + url

	r, err := http.NewRequest(method, "http://127.0.0.1:"+fmt.Sprint(common.Options.HttpServerPort)+url, nil)
	if err != nil {
		fmt.Println("WEB ERROR" + desc + ": " + err.Error())
		return false
	}

	client := http.Client{}
	resp, err := client.Do(r)
	if err != nil {
		fmt.Println("WEB ERROR" + desc + ": " + err.Error())
		return false
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("WEB ERROR" + desc + ": " + err.Error())
		return false
	}

	fmt.Println(desc + ": " + resp.Status + " - " + string(b))
	if resp.StatusCode == http.StatusOK {
		return false
	}

	return true
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

func testThreadClient(t *testing.T) {

	countThread := 100
	done := make(chan bool)

	go MainServer()
	go DataServer()

	time.Sleep(time.Second)

	fail := false
	var mutex sync.Mutex

	for i := 0; i < countThread; i++ {

		go func(n int) {
			r := creationClient()
			if !r {
				mutex.Lock()
				fail = true
				mutex.Unlock()
			}
			done <- true
		}(i)

	}

	for i := 0; i < countThread; i++ {
		<-done
	}

	require.True(t, fail == false)
}
