go-cmd:
	go install -v golang.org/x/mobile/cmd/gomobile@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

gen-proto:
	PATH=$(shell go env GOPATH)/bin:$(PATH) protoc --go_out=proto/generate --go_opt=paths=source_relative --go-grpc_out=proto/generate --go-grpc_opt=paths=source_relative proto/*.proto

gen-ent:
	go generate ./ent

regenerate: gen-proto gen-ent 
