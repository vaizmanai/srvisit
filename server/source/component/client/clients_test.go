package client

import (
	"../../common"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBasicClient(t *testing.T) {
	require.True(t, len(GetAllClientsList()) == 0)
	require.True(t, len(GetClientsList("111:111:111:111")) == 0)

	client := Client{Serial: common.RandomString(common.CodeLength), Pass: "12345"}
	client.Pid = common.GetPid(client.Serial)

	client.SetCoordinates([2]float64{3.1, 9.2})
	client.StoreClient()

	require.True(t, len(GetAllClientsList()) == 1)
	require.True(t, len(GetClientsList(client.Pid)) == 1)

	test := GetClientsList(client.Pid)[0]

	client.Pass = "54321"

	require.True(t, test.Pass == client.Pass)
	require.True(t, test.Coordinates() == [2]float64{3.1, 9.2})

	test.RemoveClient()

	require.True(t, len(GetAllClientsList()) == 0)

	require.True(t, len(GetClientsList(client.Pid)) == 0)

	countThread := 500
	countItem := 10
	done := make(chan bool)

	go func() {
		for i := 0; i < countThread; i++ {
			serial := common.RandomString(common.CodeLength)
			for j := 0; j < countItem; j++ {
				client := Client{Serial: serial, Pass: "12345"}
				client.Pid = common.GetPid(client.Serial)
				client.StoreClient()
			}
			done <- true
		}
	}()

	for i := 0; i < countThread; i++ {
		<-done
	}

	for j := 0; j < countItem; j++ {
		c := GetAllClientsList()[common.RandInt(0, countThread)]
		c.RemoveClient()
	}

	require.True(t, len(GetAllClientsList()) == countThread*countItem-countItem)
}

func TestVersionClient(t *testing.T) {
	client := Client{Serial: common.RandomString(common.CodeLength), Pass: "12345", Version: "1.0"}

	require.True(t, client.GreaterVersionThan(1.1) == false)
	require.True(t, client.GreaterVersionThan(1.0) == false)
	require.True(t, client.GreaterVersionThan(0.0) == true)
	require.True(t, client.GreaterVersionThan(0.9) == true)

	client1 := Client{Serial: common.RandomString(common.CodeLength), Pass: "12345", Version: "a1.0"}
	require.True(t, client1.GreaterVersionThan(0.0) == false)
}
