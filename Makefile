# Variables
include .env
PORT := $(shell grep -E '^PORT=' .env | cut -d '=' -f2)

# Protocol Buffer Compiler
proto-gen:
	protoc -I ./apis --go_out=apis --go-grpc_out=apis chat.proto

# gRPC UI CLI
gui-help:
	grpcui -h

gui-web:
	grpcui -plaintext localhost:50000

port:
	echo "Переменная JWT_SECRET равна $(PORT)"