package main

import (
	. "./common"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
)



func loadVNCList() {

	f, err := os.Open(FILE_VNCLIST)
	defer f.Close()
	if err == nil {
		b, err := ioutil.ReadAll(f)
		if err == nil {
			err = json.Unmarshal(b, &arrayVnc)
			if err == nil {
				defaultVnc = 0
			} else {
				LogAdd(MESS_ERROR, "Не получилось загрузить список VNC: "+fmt.Sprint(err))
			}
		} else {
			LogAdd(MESS_ERROR, "Не получилось загрузить список VNC: "+fmt.Sprint(err))
		}
	} else {
		LogAdd(MESS_ERROR, "Не получилось загрузить список VNC: "+fmt.Sprint(err))
	}
}


func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func getMyIp() string {
	int, err := net.Interfaces()
	checkError(err)

	ip := net.IPv4zero.String()
	for _, i := range int {
		if (i.Flags&net.FlagLoopback == 0) && (i.Flags&net.FlagPointToPoint == 0) && (i.Flags&net.FlagUp == 1) {
			z, err := i.Addrs()
			checkError(err)

			for _, j := range z {
				x, _, _ := net.ParseCIDR(j.String())

				if x.IsGlobalUnicast() && x.To4() != nil {
					ip = x.To4().String()
					return ip
				}
			}
		}
	}

	return ip
}

func getMyIpByExternalApi() string {
	resp, err := http.Get(URI_IPIFY_API)
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return ""
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		//todo надо мой айпи адрес как-то указать
		return ""
	}

	return string(b)
}

