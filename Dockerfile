FROM golang:1.21

RUN apt-get update -q && apt-get install -yq git lld libc++-dev libc++abi-dev clang ca-certificates mono-complete wget cmake ninja-build
RUN wget https://github.com/apple/foundationdb/releases/download/7.1.43/foundationdb-clients_7.1.43-1_amd64.deb
RUN mkdir -p /var/lib/foundationdb
RUN apt-get install -yq ./foundationdb-clients_7.1.43-1_amd64.deb
WORKDIR /fdb
RUN  git clone https://github.com/apple/foundationdb.git
RUN  cd foundationdb && git checkout tags/7.1.43
RUN CC=clang CXX=clang++ LD=lld cmake -S /fdb/foundationdb -B /fdb/cbuild_output  -D USE_LD=LLD -D USE_WERROR=ON  -D USE_LIBCXX=1 -G Ninja && ninja -C /fdb/cbuild_output

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

FROM debian:buster
RUN apt-get update -q && apt-get install -yq ca-certificates
ENV PATH='/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
COPY --from=0 /bin /bin
CMD /bin/crypto-tweet-sense