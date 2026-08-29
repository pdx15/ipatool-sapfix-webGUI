$env:GOOS='windows'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'
go build -trimpath -ldflags="-s -w -X github.com/majd/ipatool/v2/cmd.version=2.4.0-sap-unicorn.1" -o ipatool.exe .
