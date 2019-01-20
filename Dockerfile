FROM golang:latest

ARG PROJECT=XXX
ENV GOPATH /go
WORKDIR /go/src/$PROJECT

RUN go get -u github.com/LK4D4/vndr
RUN go get -u golang.org/x/lint/golint
