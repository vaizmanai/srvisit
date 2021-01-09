#!/bin/sh


mv ../build/srvisit ../build/srvisit.back
go build -o ../build/srvisit

pkill -f "srvisit"
cd ../build
./start_master.sh
./start_node.sh
