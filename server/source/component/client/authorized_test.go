package client

import (
	"../../common"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAuthorizedStatic(t *testing.T) {
	countItem := 1000
	countThread := 50
	done := make(chan bool)

	for i := 0; i < countThread; i++ {

		go func(n int) {
			email := "example@mail.com"
			if n != 0 {
				email = common.RandomString(18) + "@mail.com"
			}

			for j := 0; j < countItem; j++ {
				salt := common.RandomString(common.LengthSalt)
				token := common.RandomString(common.LengthToken)
				serial := common.RandomString(common.LengthToken)
				pass := common.RandomString(common.LengthSalt)
				a := float64(common.RandInt(10000, 99000)) / 1000.
				b := float64(common.RandInt(10000, 99000)) / 1000.
				newClient := Client{Serial: serial, Pid: common.GetPid(serial), Salt: salt, Token: token, Pass: pass, coordinates: [2]float64{a, b}}
				AddAuthorizedClient(email, &newClient)
			}
			require.True(t, len(GetAuthorizedClientList(email)) == countItem)

			DelAuthorizedClient(email, &Client{})
			require.True(t, len(GetAuthorizedClientList(email)) == countItem)

			DelAuthorizedClient(email, GetAuthorizedClientList(email)[common.RandInt(0, countItem)])
			require.True(t, len(GetAuthorizedClientList(email)) == countItem-1)

			require.True(t, len(GetAuthorizedClientList("test@mail.com")) == 0)

			done <- true
		}(i)

	}

	for i := 0; i < countThread; i++ {
		<-done
	}

	DelAuthorizedClient("root@mail.net", &Client{})

	require.True(t, len(GetAuthorizedClientList("example@mail.com")) == countItem-1)

	require.True(t, len(getContainedAllClientList()) == countItem*countThread-countThread)
}
