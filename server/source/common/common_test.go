package common

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStaticCommon(t *testing.T) {
	require.True(t, CleanPid("1:2:3:4:5:6:7") == "1234567")

	require.True(t, GetSHA256("1234567890QWERTY") == "073eaf6ba1a688d145d6c394e2dc423c0451ebf55814d7bc8837563413898742")
}