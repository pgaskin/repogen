#!/bin/bash
rm -rf ./out; go run main.go debfile.go ar.go debcontrol.go repo.go /home/patrick/Downloads/patrick-g-gpg-key-backup.asc ./in ./out -c -m "Patrick Gaskin <geek1011@outlook.com>" -w -i .5s