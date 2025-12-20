update-swag:
	swag init -g router/router.go --parseDependency true
go-cmd:
	go install github.com/swaggo/swag/cmd/swag@latest
	go install -v golang.org/x/mobile/cmd/gomobile@latest

gen-proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/miru_core_service.proto
