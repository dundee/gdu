NAME := gdu
MAJOR_VER := v5
PACKAGE := github.com/dundee/$(NAME)/$(MAJOR_VER)
CMD_GDU := cmd/gdu
VERSION := $(shell git describe --tags 2>/dev/null)
GOFLAGS ?= -buildmode=pie -trimpath -mod=readonly -modcacherw
LDFLAGS := -s -w -extldflags '-static' \
	-X '$(PACKAGE)/build.Version=$(VERSION)' \
	-X '$(PACKAGE)/build.User=$(shell id -u -n)' \
	-X '$(PACKAGE)/build.Time=$(shell LC_ALL=en_US.UTF-8 date)'

all: clean build-all man clean-uncompressed-dist shasums

run:
	go run $(PACKAGE)/$(CMD_GDU)

build:
	@echo "Version: " $(VERSION)
	mkdir -p dist
	GOFLAGS="$(GOFLAGS)" CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/$(NAME) $(PACKAGE)/$(CMD_GDU)

build-all:
	@echo "Version: " $(VERSION)
	-mkdir dist
	-CGO_ENABLED=0 gox \
		-os="darwin windows" \
		-arch="amd64" \
		-output="dist/gdu_{{.OS}}_{{.Arch}}" \
		-ldflags="$(LDFLAGS)" \
		$(PACKAGE)/$(CMD_GDU)

	-CGO_ENABLED=0 gox \
		-os="linux freebsd netbsd openbsd" \
		-output="dist/gdu_{{.OS}}_{{.Arch}}" \
		-ldflags="$(LDFLAGS)" \
		$(PACKAGE)/$(CMD_GDU)

	cd dist; GOFLAGS="$(GOFLAGS)" CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o gdu_linux_amd64 $(PACKAGE)/$(CMD_GDU)

	cd dist; CGO_ENABLED=0 GOOS=linux GOARM=5 GOARCH=arm go build -ldflags="$(LDFLAGS)" -o gdu_linux_armv5l $(PACKAGE)/$(CMD_GDU)
	cd dist; CGO_ENABLED=0 GOOS=linux GOARM=6 GOARCH=arm go build -ldflags="$(LDFLAGS)" -o gdu_linux_armv6l $(PACKAGE)/$(CMD_GDU)
	cd dist; CGO_ENABLED=0 GOOS=linux GOARM=7 GOARCH=arm go build -ldflags="$(LDFLAGS)" -o gdu_linux_armv7l $(PACKAGE)/$(CMD_GDU)
	cd dist; CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o gdu_linux_arm64 $(PACKAGE)/$(CMD_GDU)

	cd dist; for file in gdu_linux_* gdu_darwin_* gdu_netbsd_* gdu_openbsd_* gdu_freebsd_*; do tar czf $$file.tgz $$file; done
	cd dist; for file in gdu_windows_*; do zip $$file.zip $$file; done

gdu.1: gdu.1.md
	pandoc gdu.1.md -s -t man > gdu.1

man: gdu.1
	cp gdu.1 dist
	cd dist; tar czf gdu.1.tgz gdu.1

show-man:
	pandoc gdu.1.md -s -t man | man -l -

test:
	go test -v ./...

coverage:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

coverage-html: coverage
	go tool cover -html=coverage.txt

gobench:
	go test -bench=. github.com/dundee/gdu/analyze

benchmark:
	hyperfine --export-markdown=bench-cold.md \
		--prepare 'sync; echo 3 | sudo tee /proc/sys/vm/drop_caches' \
		'gdu -npc ~' 'dua ~' 'duc index ~' 'ncdu -0 -o /dev/null ~' \
		'diskus ~' 'du -hs ~' 'dust -d0 ~'
	hyperfine --export-markdown=bench-warm.md \
		--warmup 5 \
		'gdu -npc ~' 'dua ~' 'duc index ~' 'ncdu -0 -o /dev/null ~' \
		'diskus ~' 'du -hs ~' 'dust -d0 ~'

clean:
	-rm coverage.txt
	-rm -r test_dir
	-rm -r dist

clean-uncompressed-dist:
	find dist -type f -not -name '*.tgz' -not -name '*.zip' -delete

shasums:
	cd dist; sha256sum * > sha256sums.txt
	cd dist; gpg --sign --armor --detach-sign sha256sums.txt

.PHONY: run build test coverage coverage-html clean man
