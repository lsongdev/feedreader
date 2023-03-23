BINARY_NAME=bin/reader

build:
	GOARCH=arm64 GOOS=darwin go build -ldflags="-s -w" -o ${BINARY_NAME}-darwin-arm64
	GOARCH=amd64 GOOS=darwin go build -ldflags="-s -w" -o ${BINARY_NAME}-darwin-amd64
	GOARCH=amd64 GOOS=linux go build -ldflags="-s -w" -o ${BINARY_NAME}-linux-amd64
	GOARCH=amd64 GOOS=windows go build -ldflags="-s -w" -o ${BINARY_NAME}-windows-amd64.exe

clean:
	go clean
	rm ${BINARY_NAME}-darwin-arm64
	rm ${BINARY_NAME}-darwin-amd64
	rm ${BINARY_NAME}-linux-amd64
	rm ${BINARY_NAME}-windows-x64.exe
