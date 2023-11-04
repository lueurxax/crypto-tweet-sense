FROM golang:1.21

RUN apt-get update -q && apt-get install -yq ca-certificates

ARG GOPROXY
ENV \
  GO111MODULE=on \
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64

WORKDIR /go/src/github.com/lueurxax/crypto-tweet-sense/
ADD go.mod go.sum /go/src/github.com/lueurxax/crypto-tweet-sense/
RUN GOPRIVATE="git.proksy.io" go mod download -x

ADD . .

ARG VERSION
RUN go build -v -ldflags="-w -s -X main.version=${VERSION}" -o /go/bin/twitter-exp cmd/botV2/*.go

FROM debian:buster
RUN apt-get update -q && apt-get install -yq ca-certificates
ENV PATH='/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
COPY --from=0 /go/bin /go/bin
CMD /go/bin/twitter-exp
