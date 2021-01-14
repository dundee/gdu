VERSION := $(shell git describe --tags)
PACKAGES := $(shell go list ./...)

run:
	go run .

build:
	@echo "Version: " $(VERSION)
	-mkdir build
	-gox \
		-os="darwin windows" \
		-arch="amd64" \
		-output="build/{{.Dir}}_{{.OS}}_{{.Arch}}" \
		-ldflags="-s -w -X 'main.AppVersion=$(VERSION)'"
	-gox \
		-os="linux freebsd netbsd openbsd" \
		-output="build/{{.Dir}}_{{.OS}}_{{.Arch}}" \
		-ldflags="-s -w -X 'main.AppVersion=$(VERSION)'"

	cd build; GOOS=linux GOARM=5 GOARCH=arm go build -ldflags="-s -w -X 'main.AppVersion=$(VERSION)'" -o gdu_linux_armv5l ..
	cd build; GOOS=linux GOARM=6 GOARCH=arm go build -ldflags="-s -w -X 'main.AppVersion=$(VERSION)'" -o gdu_linux_armv6l ..
	cd build; GOOS=linux GOARM=7 GOARCH=arm go build -ldflags="-s -w -X 'main.AppVersion=$(VERSION)'" -o gdu_linux_armv7l ..
	cd build; GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X 'main.AppVersion=$(VERSION)'" -o gdu_linux_arm64 ..

	cd build; for file in gdu_linux_* gdu_darwin_* gdu_netbsd_* gdu_openbsd_* gdu_freebsd_*; do tar czf $$file.tgz $$file; done
	cd build; for file in gdu_windows_*; do zip $$file.zip $$file; done

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
