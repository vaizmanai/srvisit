package service

import (
	"../common"
	"../component/client"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
)

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
