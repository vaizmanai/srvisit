package profile

import (
	"../../common"
	"../contact"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGetStatic(t *testing.T) {
	profile0 := NewProfile("email0@test.net")
	profile0.Capt = "profile0"
	profile0.Pass = "pass0"
	require.True(t, GetProfile("email0@test.net") != nil)
	require.True(t, GetProfile("email0@test.net").Pass == profile0.Pass)

	profile1 := NewProfile("email1@test.net")
	profile1.Capt = "profile1"
	profile1.Pass = "pass1"
	require.True(t, GetProfile("email1@test.net") != nil)
	require.True(t, GetProfile("email1@test.net") == profile1)
	require.True(t, GetProfile("email1@test.net") != profile0)

	profile2 := NewProfile("email0@test.net")
	require.True(t, profile2 == nil)

	profile3 := NewProfile("email3@test.net")
	profile3.Capt = "profile3"
	profile3.Pass = "pass3"
	require.True(t, GetProfile("email3@test.net") != nil)
	require.True(t, GetProfile("email3@test.net").Pass == profile3.Pass)

	DelProfile("email1@test.net")
	require.True(t, GetProfile("email1@test.net") == nil)
	require.True(t, GetProfile("email0@test.net") != nil)
	require.True(t, GetProfile("email3@test.net") != nil)

	require.True(t, len(GetProfileList()) == 2)
	SaveProfiles()

	DelProfile("email3@test.net")
	DelProfile("email0@test.net")
	require.True(t, len(GetProfileList()) == 0)

	LoadProfiles()
	require.True(t, len(GetProfileList()) == 2)
	require.True(t, GetProfile("email0@test.net") != nil)
	require.True(t, GetProfile("email0@test.net").Pass == "pass0")
	require.True(t, GetProfile("email3@test.net") != nil)
	require.True(t, GetProfile("email3@test.net").Pass == "pass3")
}

func TestGetStaticContact(t *testing.T) {
	profile := NewProfile("email@test.net")
	profile.Capt = "profile"
	profile.Pass = "pass"

	for i := 0; i < 5000; i++ {
		c := &contact.Contact{Caption: "z" + fmt.Sprint(i), Pid: fmt.Sprint(i), Id: contact.GetNewId(profile.Contacts)}
		c.Digest = common.RandomString(32)
		c.Salt = common.RandomString(64)
		c.Type = common.RandomString(16)
		c.Next = profile.Contacts
		profile.Contacts = c
	}
	x := contact.GetContactByPid(profile.Contacts, "345")
	x.Digest = "QWE"
	x.Salt = "QAZ"
	x.Type = "ZXC"
	SaveProfiles()
	DelProfile("email@test.net")

	require.True(t, GetProfile("email@test.net") == nil)

	LoadProfiles()
	test := GetProfile("email@test.net")
	require.True(t, test != nil)

	c := contact.GetContactByPid(test.Contacts, "345")
	require.True(t, c != nil)
	require.True(t, c.Pid == "345")
	require.True(t, c.Caption == "z345")
	require.True(t, c.Pid == "345")

	require.True(t, c.Digest == "QWE")
	require.True(t, c.Salt == "QAZ")
	require.True(t, c.Type == "ZXC")
}
