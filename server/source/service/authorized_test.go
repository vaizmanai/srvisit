package service

import (
    "../common"
    "github.com/stretchr/testify/require"
    "testing"
)

func TestGetStatic(t *testing.T) {
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
            require.True(t, len(GetListAuthorizedClient(email)) == countItem)

            DelAuthorizedClient(email, &Client{})
            require.True(t, len(GetListAuthorizedClient(email)) == countItem)

            DelAuthorizedClient(email, GetListAuthorizedClient(email)[common.RandInt(0, countItem)])
            require.True(t, len(GetListAuthorizedClient(email)) == countItem-1)

            require.True(t, len(GetListAuthorizedClient("test@mail.com")) == 0)

            done <- true
        }(i)

    }

    for i := 0; i < countThread; i++ {
        <-done
    }

    require.True(t, len(GetListAuthorizedClient("example@mail.com")) == countItem-1)
}