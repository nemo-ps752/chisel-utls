VERSION=$(shell git describe --abbrev=0 --tags)
BUILD=$(shell git rev-parse HEAD)
DIRBASE=./build
DIR=${DIRBASE}

LDFLAGS=-ldflags "-extldflags -static"

GOFILES=`go list ./...`
GOFILESNOTEST=`go list ./... | grep -v test`

# Make Directory to store executables
$(shell mkdir -p ${DIR})

all:
	@goreleaser build --skip-validate --single-target --config .github/goreleaser.yml

freebsd: lint
	env CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 garble -literals -tiny -seed=random build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/utunnel-freebsd_amd64 .

linux: lint
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 garble -literals -tiny -seed=random build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/utunnel-linux_amd64 .

windows: lint
	env CGO_ENABLED=1 GOOS=windows GOARCH=amd64 garble -literals -tiny -seed=random build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/utunnel-windows_amd64 .

darwin:
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 garble -literals -tiny -seed=random build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/utunnel-darwin_amd64 .

openwrt:
	env CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat garble -literals -tiny -seed=random build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/utunnel-openwrt .

docker:
	@docker build .

dep: ## Get the dependencies
	@go get -u github.com/goreleaser/goreleaser
	@go get -u github.com/boumenot/gocover-cobertura
	@go get -v -d ./...
	@go get -u all
	@go mod tidy

lint: ## Lint the files
	@go fmt ${GOFILES}
	@go vet ${GOFILESNOTEST}

test: ## Run unit tests
	@go test -coverprofile=${DIR}/coverage.out -race -short ${GOFILESNOTEST}
	@go tool cover -html=${DIR}/coverage.out -o ${DIR}/coverage.html
	@gocover-cobertura < ${DIR}/coverage.out > ${DIR}/coverage.xml

release: lint test
	goreleaser release --config .github/goreleaser.yml

clean:
	rm -rf ${DIRBASE}/*

.PHONY: all freebsd linux windows docker dep lint test release clean