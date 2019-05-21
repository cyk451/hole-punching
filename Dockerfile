
FROM golang:1.12.5-alpine3.9 AS builder
MAINTAINER cyk451@gmail.com

RUN apk add --no-cache git
# Copy the code from the host and compile it
WORKDIR home/hole-punching
COPY . ./
# RUN GOPATH=$GOPATH:$PWD go get -d ./...
RUN GOPATH=$GOPATH:$PWD CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -a -installsuffix nocgo -o /server ./server
RUN GOPATH=$GOPATH:$PWD CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -a -installsuffix nocgo -o /client ./client
# RUN upx /apmd

FROM alpine
# COPY ./ipipdb.datx /ipipdb.datx
COPY --from=builder /server /server
COPY --from=builder /client /client
CMD ["/server"]
