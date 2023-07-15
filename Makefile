.PHONY: windows linux

GO111MODULE = on
GOARCH = amd64
BIN_PATH = /opt/srvisit

all: windows linux

windows: windows_server

linux: linux_server

windows_server:
	set GOOS = windows
	go build -o build/server.exe github.com/vaizmanai/srvisit/cmd/server

linux_server:
	set GOOS = linux
	go build -o build/server github.com/vaizmanai/srvisit/cmd/server

windows_clean:
	cmd /r del /s /q "*.tmp"; echo ok
	cmd /r del /s /q "*.txt"; echo ok
	cmd /r del /s /q "options.json"; echo ok
	cmd /r del /s /q "profiles.json"; echo ok

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

clean: windows_clean
