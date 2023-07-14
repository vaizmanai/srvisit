# Server Communicator for reVisit
[![Go Report Card](https://goreportcard.com/badge/github.com/vaizmanai/srvisit)](https://goreportcard.com/report/github.com/vaizmanai/srvisit)
[![codecov](https://codecov.io/gh/vaizmanai/srvisit/branch/master/graph/badge.svg)](https://codecov.io/gh/vaizmanai/srvisit)
[![Build Status](https://travis-ci.org/vaizmanai/srvisit.svg?branch=master)](https://travis-ci.org/vaizmanai/srvisit)

## Notes

Server for managing VNC clients, storing list of contacts, passing through NAT.

Public version supports work through additional data servers
![Screen map](https://vaizman.ru/revisit/p1.jpg)

Supports statistics per day/week/month/year
![Statistics](https://vaizman.ru/revisit/p2.jpg)

## Building

Building windows regular version:

```
make windows
```

Building linux regular version:

```
make linux
```

Install linux regular version:

```
make linux_install
```

Install linux master + data versions:

```
make linux_install_master
```

***
public site
https://rvisit.net

client side
https://github.com/vaizmanai/clvisit

ui client side
https://github.com/vaizmanai/uivisit
