package client

import (
	"../../common"
	. "../../component/profile"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestContainedStatic(t *testing.T) {
	countItem := 500
	countThread := 100
	done := make(chan bool)

	for i := 0; i < countThread; i++ {

		go func(n int) {
			pid := common.GetPid(common.RandomString(common.LengthToken))

			email := "test@mail.com"
			profile := Profile{Email: email}
			AddContainedProfile(pid, &profile)

			for j := 1; j < countItem; j++ {
				email := common.RandomString(18) + "@mail.com"
				profile := Profile{Email: email}
				AddContainedProfile(pid, &profile)
			}
			require.True(t, len(GetContainedProfileList(pid)) == countItem)

			DelContainedProfile(pid, &Profile{})
			require.True(t, len(GetContainedProfileList(pid)) == countItem)

			DelContainedProfile(pid, GetContainedProfileList(pid)[common.RandInt(0, countItem)])
			require.True(t, len(GetContainedProfileList(pid)) == countItem-1)

			require.True(t, len(GetContainedProfileList("1234567890")) == 0)
			done <- true
		}(i)

	}

	for i := 0; i < countThread; i++ {
		<-done
	}

	DelContainedProfile("000:000:000", &Profile{})

	require.True(t, len(getContainedAllProfileList()) == (countItem-1)*countThread)
}
