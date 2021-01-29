NAME := gdu
PACKAGE := github.com/dundee/$(NAME)
VERSION := $(shell git describe --tags)
GOFLAGS ?= -buildmode=pie -trimpath -mod=readonly -modcacherw
LDFLAGS := "-s -w -extldflags '-static' \
	-X '$(PACKAGE)/build.Version=$(VERSION)' \
	-X '$(PACKAGE)/build.User=$(shell id -u -n)' \
	-X '$(PACKAGE)/build.Time=$(shell LC_ALL=en_US.UTF-8 date)'"

run:
	go run .

build:
	@echo "Version: " $(VERSION)
	mkdir -p dist
	GOFLAGS="$(GOFLAGS)" CGO_ENABLED=0 go build -a -ldflags=$(LDFLAGS) -o dist/$(NAME) .

build-all:
	@echo "Version: " $(VERSION)
	-mkdir dist
	-CGO_ENABLED=0 gox \
		-os="darwin windows" \
		-arch="amd64" \
		-output="dist/{{.Dir}}_{{.OS}}_{{.Arch}}" \
		-ldflags=$(LDFLAGS)

	-CGO_ENABLED=0 gox \
		-os="linux freebsd netbsd openbsd" \
		-output="dist/{{.Dir}}_{{.OS}}_{{.Arch}}" \
		-ldflags=$(LDFLAGS)

	cd dist; GOFLAGS="$(GOFLAGS)" CGO_ENABLED=0 go build -a -ldflags=$(LDFLAGS) -o gdu_linux_amd64 ..

	cd dist; CGO_ENABLED=0 GOOS=linux GOARM=5 GOARCH=arm go build -ldflags=$(LDFLAGS) -o gdu_linux_armv5l ..
	cd dist; CGO_ENABLED=0 GOOS=linux GOARM=6 GOARCH=arm go build -ldflags=$(LDFLAGS) -o gdu_linux_armv6l ..
	cd dist; CGO_ENABLED=0 GOOS=linux GOARM=7 GOARCH=arm go build -ldflags=$(LDFLAGS) -o gdu_linux_armv7l ..
	cd dist; CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags=$(LDFLAGS) -o gdu_linux_arm64 ..

	cd dist; for file in gdu_linux_* gdu_darwin_* gdu_netbsd_* gdu_openbsd_* gdu_freebsd_*; do tar czf $$file.tgz $$file; done
	cd dist; for file in gdu_windows_*; do zip $$file.zip $$file; done

test:
	go test -v ./...

coverage:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

coverage-html: coverage
	go tool cover -html=coverage.txt

gobench:
	go test -bench=. ./...

benchmark:
	hyperfine --export-markdown=bench-cold.md \
		--prepare 'sync; echo 3 | sudo tee /proc/sys/vm/drop_caches' \
		'gdu -npc ~' 'dua ~' 'duc index ~' 'ncdu -0 -o /dev/null ~' \
    	'diskus -b ~' 'du -hs ~' 'dust -d0 ~'
	hyperfine --export-markdown=bench-warm.md \
		--warmup 5 \
		'gdu -npc ~' 'dua ~' 'duc index ~' 'ncdu -0 -o /dev/null ~' \
    	'diskus -b ~' 'du -hs ~' 'dust -d0 ~'

clean:
	-rm coverage.txt
	-rm -r test_dir
	-rm -r dist

clean-uncompressed-dist:
	find dist -type f -not -name '*.tgz' -not -name '*.zip' -delete

.PHONY: run build test coverage coverage-html clean
