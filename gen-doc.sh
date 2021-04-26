#!/bin/bash

# Generates Go documentation and exports it as static HTML.

rm -rf doc
ln -s RULES/ src
godoc -goroot=`pwd` -http=:8080 &
GODOCPID=$!
sleep 1
wget -r -np -N -E -p -k -e robots=off http://localhost:8080/pkg/
mv localhost:8080/ doc/
kill -9 $GODOCPID
rm src