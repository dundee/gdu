VERSION := $(shell git describe --tags)
PACKAGES := $(shell go list ./...)

run:
	go run .

build:
	@echo "Version: " $(VERSION)
	-mkdir build
	cd build; GOOS=linux GOARCH=arm go build -ldflags="-s -w -X 'main.AppVersion=$(VERSION)'" -o gdu-linux-arm ..; tar czf gdu-linux-arm.tgz gdu-linux-arm
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

benchnmark:
	go test -bench=. $(PACKAGES)

clean:
	-rm coverage.txt
	-rm -r test_dir
	-rm -r build
	-sudo rm -r obj-x86_64-linux-gnu on *.deb

.PHONY: run build test coverage clean