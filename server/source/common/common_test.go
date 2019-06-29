package common

import (
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
	"time"
)

func TestStaticCommon(t *testing.T) {
	require.True(t, CleanPid("1:2:3:4:5:6:7") == "1234567")

	require.True(t, GetSHA256("1234567890QWERTY") == "073eaf6ba1a688d145d6c394e2dc423c0451ebf55814d7bc8837563413898742")

	require.True(t, GetPid("ABCDEFGHIJKLMNOPQRSTUVWXYZ") == "531:281:720:456")

	rand.Seed(time.Now().UTC().UnixNano())
	r := RandInt(20, 30)
	require.True(t, r >= 20 && r < 30)

	require.True(t, len(RandomString(128)) == 128)

	sent, _ := SendEmail("", "")
	require.True(t, sent == false)
}
