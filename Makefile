proto-gen:
	protoc -I ./apis --go_out=apis --go-grpc_out=apis chat.proto