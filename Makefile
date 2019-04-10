all: ./bin/client ./bin/server

./bin/client: src/client/main.go
	go build -o ./bin/client ./src/client

./bin/server: src/server/main.go
	go build -o ./bin/server ./src/server
