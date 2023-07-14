.PHONY: windows linux

GO111MODULE = on
GOARCH = amd64

all: windows linux

windows: windows_server

linux: linux_server

windows_server:
	set GOOS = windows
	@go build -o build/server.exe srvisit/cmd/server

linux_server:
	set GOOS = linux
	@go build -o build/server srvisit/cmd/server

windows_clean:
	cmd /r del /s /q "*.tmp"; echo ok
	cmd /r del /s /q "*.txt"; echo ok
	cmd /r del /s /q "options.json"; echo ok
	cmd /r del /s /q "profiles.json"; echo ok

linux_clean:
	echo todo

linux_start_master:
	systemctl start srvisit-master
	systemctl start srvisit-node

linux_stop_master:
	systemctl stop srvisit-master
	systemctl stop srvisit-node

linux_start:
	systemctl start srvisit

linux_stop:
	systemctl stop srvisit

linux_upgrade: git_pull linux_server linux_stop linux_stop_master
	cp -rf build/server /opt/svisit/
	make linux_start_master
	make linux_start
	echo finished

linux_install_common: git_pull linux_server
	cp -rf init/*.service /etc/systemd/system/
	mkdir -p /opt/srvisit
	cp -rf build/resource /opt/srvisit/
	cp -rf build/vnc.json /opt/srvisit/vnc.json
	cp -rf build/server /opt/svisit/

linux_install_master: linux_install_common
	systemctl enable srvisit-master
	systemctl enable srvisit-node
	make linux_start_master

linux_install: linux_install_common
	systemctl enable srvisit
	systemctl start srvisit
	make linux_start

git_pull:
	git pull --rebase

backup:
	echo todo

clean: windows_clean linux_clean
