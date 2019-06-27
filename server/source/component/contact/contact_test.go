package contact

import (
	"../../common"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

const countItem = 5000

func TestGetStatic(t *testing.T) {
	t.Parallel()

	origin := &Contact{Id: GetNewId(nil)}

	c := &Contact{Caption: "c0", Pid: "111:111:111:000", Id: GetNewId(origin)}
	c.Inner = origin
	origin = c

	c = &Contact{Caption: "c1", Pid: "111:111:111:111", Id: GetNewId(origin)}
	c.Next = origin
	origin = c

	c = &Contact{Caption: "c2", Pid: "111:111:111:222", Id: GetNewId(origin)}
	c.Inner = origin
	origin = c

	c = &Contact{Caption: "c3", Pid: "111:111:111:333", Id: GetNewId(origin)}
	c.Next = origin
	origin = c

	c = &Contact{Caption: "c4", Pid: "111:111:111:444", Id: GetNewId(origin)}
	c.Inner = origin
	origin = c

	c = &Contact{Caption: "c5", Pid: "111:111:111:555", Id: GetNewId(origin)}
	c.Next = origin
	origin = c

	c = &Contact{Caption: "c6", Pid: "111:111:111:666", Id: GetNewId(origin)}
	c.Inner = origin
	origin = c

	c = &Contact{Caption: "c7", Pid: "111:111:111:777", Id: GetNewId(origin)}
	c.Next = origin
	origin = c

	c = &Contact{Caption: "c8", Pid: "111:111:111:888", Id: GetNewId(origin)}
	c.Inner = origin
	origin = c

	c = &Contact{Caption: "c9", Pid: "111:111:111:999", Id: GetNewId(origin)}
	c.Next = origin
	origin = c

	testContact7 := GetContactByPid(origin, common.CleanPid("111:111:111:777"))

	require.True(t, testContact7 != nil)
	require.True(t, (*testContact7).Pid == "111:111:111:777")

	require.True(t, DelContact(nil, 123) == nil)
	require.True(t, GetContact(origin, 0) == nil)
	require.True(t, GetNewId(nil) == 1)
}

func TestNext(t *testing.T) {

	t.Parallel()

	origin := new(Contact)

	for i := 0; i < countItem; i++ {
		c := &Contact{Caption: "t" + fmt.Sprint(i), Pid: fmt.Sprint(i), Id: GetNewId(origin)}
		c.Next = origin
		origin = c
	}

	testContact456 := GetContactByPid(origin, "456")
	require.True(t, testContact456 != nil)
	require.True(t, (*testContact456).Pid == "456")

	//removing contact
	origin = DelContact(origin, testContact456.Id)
	testContact456 = GetContactByPid(origin, "456")
	require.True(t, testContact456 == nil)

	//compare by pid and by id
	testContact345 := GetContactByPid(origin, "345")
	require.True(t, testContact345 != nil)
	require.True(t, testContact345 == GetContact(origin, testContact345.Id))
}

func TestInner(t *testing.T) {

	t.Parallel()

	origin := new(Contact)

	for i := 0; i < countItem; i++ {
		c := &Contact{Caption: "t" + fmt.Sprint(i), Pid: fmt.Sprint(i), Id: GetNewId(origin)}
		c.Inner = origin
		origin = c
	}

	testContact345 := GetContactByPid(origin, "345")
	require.True(t, testContact345 != nil)
	require.True(t, (*testContact345).Pid == "345")

	//removing contact
	origin = DelContact(origin, testContact345.Id)
	testContact345 = GetContactByPid(origin, "345")
	require.True(t, testContact345 == nil)

	//compare by pid and by id
	testContact456 := GetContactByPid(origin, "456")
	require.True(t, testContact456 != nil)
	require.True(t, testContact456 == GetContact(origin, testContact456.Id))
}
