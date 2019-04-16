FROM alpine

WORKDIR /go/src/app
ADD ./bin ./bin

CMD ["/go/src/app/bin/server"]
