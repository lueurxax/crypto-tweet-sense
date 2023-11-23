ARG FDB_VERSION=7.3.27
FROM foundationdb/foundationdb:${FDB_VERSION} as fdb
FROM golang:1.21
ARG FDB_VERSION

WORKDIR /tmp

RUN apt-get update
# dnsutils is needed to have dig installed to create cluster file
RUN apt-get install -y --no-install-recommends ca-certificates dnsutils

RUN wget "https://github.com/apple/foundationdb/releases/download/${FDB_VERSION}/foundationdb-clients_${FDB_VERSION}-1_amd64.deb"
RUN dpkg -i foundationdb-clients_${FDB_VERSION}-1_amd64.deb


ARG GOPROXY
ENV \
  GO111MODULE=on \
  CGO_ENABLED=1 \
  GOOS=linux \
  GOARCH=amd64

WORKDIR /go/src/github.com/lueurxax/crypto-tweet-sense/
ADD go.mod go.sum /go/src/github.com/lueurxax/crypto-tweet-sense/
RUN go mod download -x

ADD . .

ARG VERSION
RUN go build -v -ldflags="-w -s -X main.version=${VERSION}" -o /bin/crypto-tweet-sense cmd/botV2/*.go

CMD /bin/crypto-tweet-sense