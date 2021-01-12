VERSION := $(shell git describe --tags)
PACKAGES := $(shell go list ./...)

run:
	go run .

build:
	@echo "Version: " $(VERSION)
	-mkdir build
	cd build; GOOS=linux GOARM=5 GOARCH=arm go build -ldflags="-s -w -X 'main.AppVersion=$(VERSION)'" -o gdu-linux-armv5l ..; tar czf gdu-linux-armv5l.tgz gdu-linux-armv5l
	cd build; GOOS=linux GOARM=6 GOARCH=arm go build -ldflags="-s -w -X 'main.AppVersion=$(VERSION)'" -o gdu-linux-armv6l ..; tar czf gdu-linux-armv6l.tgz gdu-linux-armv6l
	cd build; GOOS=linux GOARM=7 GOARCH=arm go build -ldflags="-s -w -X 'main.AppVersion=$(VERSION)'" -o gdu-linux-armv7l ..; tar czf gdu-linux-armv7l.tgz gdu-linux-armv7l
	cd build; GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X 'main.AppVersion=$(VERSION)'" -o gdu-linux-arm64 ..; tar czf gdu-linux-arm64.tgz gdu-linux-arm64
	cd build; GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X 'main.AppVersion=$(VERSION)'" -o gdu-linux-amd64 ..; tar czf gdu-linux-amd64.tgz gdu-linux-amd64
	cd build; GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X 'main.AppVersion=$(VERSION)'" -o gdu-windows-amd64.exe ..; zip gdu-windows-amd64.zip gdu-windows-amd64.exe
	cd build; GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X 'main.AppVersion=$(VERSION)'" -o gdu-darwin-amd64 ..; tar czf gdu-darwin-amd64.tgz gdu-darwin-amd64

build-deb:
	docker build -t debian_go .
	docker run -v $(CURDIR)/..:/xxx -w /xxx/gdu debian_go bash -c "dpkg-buildpackage"

test:
	go test -v $(PACKAGES)

coverage:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic $(PACKAGES)

coverage-html: coverage
	go tool cover -html=coverage.txt

benchnmark:
	go test -bench=. $(PACKAGES)

clean:
	-rm coverage.txt
	-rm -r test_dir
	-rm -r build
	-sudo rm -r obj-x86_64-linux-gnu on *.deb

.PHONY: run build test coverage coverage-html clean
