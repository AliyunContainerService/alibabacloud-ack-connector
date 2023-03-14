PACKAGE=github.com/alibaba/alibabacloud-ack-connector/common
CURRENT_DIR=$(shell pwd)
DIST_DIR=${CURRENT_DIR}/dist
BIN_NAME=alibabacloud-ack-connector
IMAGE_NAME?=registry.cn-hangzhou.aliyuncs.com/acs/alibabacloud-ack-connector


VERSION=$(shell cat ${CURRENT_DIR}/VERSION)
BUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(shell git rev-parse --short HEAD)
GIT_TAG=$(shell if [ -z "`git status --porcelain`" ]; then git describe --exact-match --tags HEAD 2>/dev/null; fi)
GIT_TREE_STATE=$(shell if [ -z "`git status --porcelain`" ]; then echo "clean" ; else echo "dirty"; fi)
IMAGE_TAG=$(shell git describe --tags --long|awk -F '-' '{print $1"."$2"-"$3"-alibabacloud"}')

override LDFLAGS += \
  -X ${PACKAGE}.version=${VERSION} \
  -X ${PACKAGE}.buildDate=${BUILD_DATE} \
  -X ${PACKAGE}.gitCommit=${GIT_COMMIT} \
  -X ${PACKAGE}.gitTreeState=${GIT_TREE_STATE}\
  -X ${PACKAGE}.gitTag=${GIT_TAG}


.PHONY: build
build:
	GO111MODULE=on CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build -v -ldflags '${LDFLAGS}' -o ${DIST_DIR}/${BIN_NAME} cmd/*.go

.PHONY: release
release:
	make BIN_NAME=alibabacloud-ack-connector-darwin-amd64 GOOS=darwin GOARCH=amd64 build
	make BIN_NAME=alibabacloud-ack-connector-darwin-arm64 GOOS=darwin GOARCH=arm64 build
	make BIN_NAME=alibabacloud-ack-connector-linux-amd64 GOOS=linux GOARCH=amd64 build
	make BIN_NAME=alibabacloud-ack-connector-linux-arm64 GOOS=linux GOARCH=arm64 build


.PHONY: docker-build
docker-build:
	docker build -t ${IMAGE_NAME}:${GIT_TAG}-${GIT_COMMIT}-alibabacloud . -f Dockerfile


.PHONY: docker-push
docker-push:
	docker push  ${IMAGE_NAME}:${IMAGE_TAG}