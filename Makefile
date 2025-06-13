.PHONY: all res build run gotool clean help

BINARY="ClashTray.exe"

all: gotool res build

build:
ifeq ($(OS),Windows_NT)
	@cmd /c if not exist build mkdir build
	set CGO_ENABLED=0& set GOOS=windows& set GOARCH=amd64& go build -o build/ClashTray.exe -trimpath -ldflags "-H windowsgui -s -w"
else
	@mkdir -p build
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o build/ClashTray.exe -trimpath -ldflags "-H windowsgui -s -w"
endif

res:
	go-winres make

run:
	@go run ./

gotool:
	go fmt ./...
	go vet ./...

clean:
ifeq ($(OS),Windows_NT)
	@if exist build rmdir /s /q build
else
	@rm -rf build
endif

help:
	@echo "make - 格式化 Go 代码, 并编译生成二进制文件"
	@echo "make build - 编译 Go 代码, 生成二进制文件"
	@echo "make run - 直接运行 Go 代码"
	@echo "make clean - 移除二进制文件和 vim swap files"
	@echo "make gotool - 运行 Go 工具 'fmt' and 'vet'"
	@echo "make res - 生成资源文件"
