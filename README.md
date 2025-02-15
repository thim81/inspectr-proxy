### Build Inspectr binary

1. Clone the Repository:

```bash
git clone https://github.com/thim81/inspectr-proxy.git
cd inspectr-proxy
```

2. Install Go: If you don't have Go installed, download and install it from https://go.dev/dl/. Set up your GOPATH and GOROOT environment variables.

3. Resolve Dependencies:

```bash
go mod tidy
go mod vendor
```

4. Build from source as binary

```bash
go build
```

5. Run the binary:

```bash
./inspectr --listen=":8080" --backend="http://localhost:4005" --broadcast="http://localhost:4004/sse" --print=true --app=true --appPort="9999"
curl http://localhost:8080/
```

By default the Inspectr Proxy listens on port 8080.