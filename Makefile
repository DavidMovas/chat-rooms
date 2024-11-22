# Protocol Buffer Compiler
proto-gen:
	protoc -I ./apis --go_out=apis --go-grpc_out=apis chat.proto

# gRPC UI CLI
gui-help:
	grpcui -h

gui-web:
	grpcui -plaintext localhost:50000

# Docker Redis
redis-up:
	docker run -d --name chat-rooms-redis -p 6379:6379 redis

redis-down:
	docker rm -f chat-rooms-redis

# Linter and formatter
go-fmt:
	 gofumpt -l -w .

go-lint:
	golangci-lint run ./...

# Git hooks
before-push:
	go mod tidy && \
	gofumpt -l -w . && \
	go build ./... && \
	golangci-lint run ./... && \
	go test ./integration_tests/...