build:
	GOOS=windows GOARCH=amd64 go build -ldflags -H=windowsgui -o shipdon.exe .
