package api

import (
	"../common"
	"../component/client"
	"../component/profile"
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"strings"
	"testing"
)

type TestWebClient struct {
	header http.Header
	code   int
	test   []byte
}

func (client *TestWebClient) Header() http.Header {
	if client.header == nil {
		client.header = http.Header{}
	}
	return client.header
}

func (client *TestWebClient) Write(b []byte) (int, error) {
	client.test = b
	return len(b), nil
}

func (client *TestWebClient) WriteHeader(statusCode int) {
	client.code = statusCode
}

func (client *TestWebClient) StatusCode() int {
	return client.code
}

func TestStaticApi(t *testing.T) {
	w := &TestWebClient{}
	r := &http.Request{}

	w.WriteHeader(123)
	require.True(t, w.StatusCode() == 123)

	n, err := w.Write([]byte("12345"))
	require.True(t, n == 5 && err == nil)

	w.Header().Set("test", "TEST")
	require.True(t, w.Header().Get("test") == "TEST")

	//------------

	common.LogAdd(common.MessFull, "preparing for deletion")
	HandleGetLog(w, r)
	require.True(t, strings.Contains(string(w.test), "preparing for deletion"))

	HandleDelLog(w, r)
	HandleGetLog(w, r)
	require.True(t, !strings.Contains(string(w.test), "preparing for deletion"))

	profile.NewProfile("test@mail.net")
	HandleGetProfileList(w, r)
	fmt.Println(string(w.test))
	require.True(t, string(w.test) == `[{"Email":"test@mail.net","Pass":"","Contacts":null,"Capt":"","Tel":"","Logo":""}]`)

	test := client.Client{Pid: "1234567890"}
	test.StoreClient()
	HandleGetClientsList(w, r)
	fmt.Println(string(w.test))
	require.True(t, string(w.test) == `[{"Serial":"","Pid":"1234567890","Pass":"","Version":"","Salt":"","Profile":null,"Token":"","Conn":null,"Code":""}]`)

	//------------

	HandleGetClient(w, r, &test)
	fmt.Println(string(w.test))
	require.True(t, string(w.test) == `{"Serial":"","Pid":"1234567890","Pass":"","Version":"","Salt":"","Profile":null,"Token":"","Conn":null,"Code":""}`)

}
