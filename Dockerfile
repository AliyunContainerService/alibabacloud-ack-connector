FROM docker.io/library/golang:1.18.6 AS builder

RUN echo 'deb http://mirrors.aliyun.com/debian/ buster main non-free contrib' > /etc/apt/sources.list
RUN echo 'deb-src http://mirrors.aliyun.com/debian/ buster main non-free contrib' >> /etc/apt/sources.list
RUN echo 'deb http://mirrors.aliyun.com/debian-security buster/updates main' >> /etc/apt/sources.list
RUN echo 'deb-src http://mirrors.aliyun.com/debian-security buster/updates main' >> /etc/apt/sources.list
RUN echo 'deb http://mirrors.aliyun.com/debian/ buster-updates main non-free contrib' >> /etc/apt/sources.list
RUN echo 'deb-src http://mirrors.aliyun.com/debian/ buster-updates main non-free contrib' >> /etc/apt/sources.list
RUN echo 'deb http://mirrors.aliyun.com/debian/ buster-backports main non-free contrib' >> /etc/apt/sources.list
RUN echo 'deb-src http://mirrors.aliyun.com/debian/ buster-backports main non-free contrib' >> /etc/apt/sources.list

ENV GOPROXY=https://goproxy.cn,direct

RUN apt-get update && apt-get install -y ca-certificates \
    make \
    git \
    curl \
    mercurial

ARG PACKAGE=github.com/alibaba/alibabacloud-ack-connector

RUN mkdir -p /go/src/${PACKAGE}
WORKDIR /go/src/${PACKAGE}

COPY . .
# Build
RUN GOPRIVATE="*.alibaba-inc.com" go mod tidy
RUN  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o agent_bin -ldflags "-X main.GitCommit=$(git rev-list -1 HEAD)" github.com/alibaba/alibabacloud-ack-connector/cmd

# Copy the alibabacloud-ack-connector into a thin image
FROM registry.cn-hangzhou.aliyuncs.com/acs/alpine:3.16-update

USER 65534

WORKDIR /
COPY --from=builder /go/src/github.com/alibaba/alibabacloud-ack-connector/agent_bin /usr/bin/agent
ENTRYPOINT ["/usr/bin/agent"]
