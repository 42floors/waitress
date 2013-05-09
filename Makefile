SHELL = /bin/sh

all:
	make clean
	make build

clean:
	go clean

build:
	go install

install:
	mv waitress /usr/local/bin/waitress

uninstall:
	rm /usr/local/bin/waitress
