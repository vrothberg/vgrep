FROM golang:latest

ARG PROJECT=XXX
ENV GOPATH /go
WORKDIR /go/src/$PROJECT

RUN apt-get update && apt-get install -y bats less ripgrep && rm -rf /var/lib/apt/lists/*
