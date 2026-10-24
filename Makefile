.PHONY: all windows linux windows_server linux_server test windows_clean clean

GOARCH ?= amd64
BIN_PATH = /opt/srvisit

ifeq ($(OS),Windows_NT)
SHELL := cmd.exe
GO_WINDOWS = set "GOOS=windows"&& set "GOARCH=$(GOARCH)"&& set "CGO_ENABLED=0"&&
GO_LINUX = set "GOOS=linux"&& set "GOARCH=$(GOARCH)"&& set "CGO_ENABLED=0"&&
else
GO_WINDOWS = GOOS=windows GOARCH=$(GOARCH) CGO_ENABLED=0
GO_LINUX = GOOS=linux GOARCH=$(GOARCH) CGO_ENABLED=0
endif

all: windows linux

windows: windows_server

linux: linux_server

windows_server:
	$(GO_WINDOWS) go build -o build/server.exe ./cmd/server

linux_server:
	$(GO_LINUX) go build -o build/server ./cmd/server

test:
	go vet ./...
	go test -timeout 3m ./...

windows_clean: clean

linux_uninstall: linux_stop_master linux_stop
	systemctl disable srvisit-master; echo ok
	systemctl disable srvisit-node; echo ok
	systemctl disable srvisit; echo ok
	rm -rf /etc/cron.daily/srvisit-backup
	rm -rf /etc/systemd/system/srvisit*
	rm -rf ${BIN_PATH}

linux_start_master:
	systemctl start srvisit-master
	systemctl start srvisit-node

linux_stop_master:
	systemctl stop srvisit-master; echo ok
	systemctl stop srvisit-node; echo ok

linux_start:
	systemctl start srvisit

linux_stop:
	systemctl stop srvisit; echo ok

linux_update: linux_server linux_stop linux_stop_master
	cp -rf build/server ${BIN_PATH}/
	make linux_start_master
	make linux_start
	echo finished

linux_install_common: linux_server
	cp -rf init/srvisit-backup /etc/cron.daily/
	cp -rf init/*.service /etc/systemd/system/
	mkdir -p ${BIN_PATH}
	mkdir -p ${BIN_PATH}/backup
	cp -rf build/resource ${BIN_PATH}/
	cp -rf build/vnc.json ${BIN_PATH}/vnc.json
	cp -rf build/server ${BIN_PATH}/

linux_install_master: linux_install_common
	systemctl enable srvisit-master
	systemctl enable srvisit-node
	make linux_start_master

linux_install: linux_install_common
	systemctl enable srvisit
	systemctl start srvisit
	make linux_start

clean:
ifeq ($(OS),Windows_NT)
	powershell -NoProfile -Command "Remove-Item -LiteralPath 'build/server.exe','build/server' -Force -ErrorAction SilentlyContinue"
else
	rm -f build/server.exe build/server
endif
