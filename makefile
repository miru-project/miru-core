update-swag:
	swag init -g router/router.go --parseDependency true
go-cmd:
	go install github.com/swaggo/swag/cmd/swag@latest
	go install -v golang.org/x/mobile/cmd/gomobile@latest
