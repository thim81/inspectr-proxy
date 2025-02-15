# Inspectr Proxy

**Inspectr Proxy** is a high-performance HTTP proxy that captures and inspects every incoming request and
outgoing response. It can forward requests to a backend service and capture the API requests and webhook
events. The proxy can optionally log details to the console, broadcast events via HTTP POST to a remote destination, and
even serve an embedded Inspectr UI with real‑time Server‑Sent Events (SSE) for live monitoring.

<img src="https://raw.githubusercontent.com/thim81/inspectr/main/assets/inspectr-app.png" alt="Request Inspectr" width="80%">

For more information on Inspectr visit the [Inspectr documentation](https://github.com/thim81/inspectr).

## Features

- **HTTP Proxy:** Forwards incoming requests to a configured backend service and returns the backend response.
- **Request & Response Inspection:** Captures full request and response details (including headers, payload, query
  parameters, status codes, etc.).
- **Webhook Inspection:** Receive incoming webhook requests for easier debugging (including headers, payload, query
  parameters, status codes, etc.).
- **Console Logging:** Optionally prints a color‑coded summary of each request/response to the terminal.
- **Remote Broadcasting:** Optionally sends the captured data via HTTP POST to a remote broadcast destination (e.g. for
  central logging or further processing).
- **Embedded Inspectr UI:** (Optional) Serves an embedded Inspectr App UI with SSE endpoints for real‑time monitoring.

## Installation

### Download Binary

Alternatively, you can download pre‑compiled binaries from
the [Releases](https://github.com/thim81/inspectr-proxy/releases) page.

### Build from Source

Make sure you have Go 1.16 or newer installed.

Clone the repository and build the binary:

```bash
git clone https://github.com/yourusername/inspectr-proxy.git
cd inspectr-proxy
bash setup.sh
go build -o inspectr
```

## Usage

Once downloaded (or built), you can run the Inspectr proxy from the command line with various configuration options. For
example:

```bash
./inspectr --listen=":8080" --backend="http://localhost:3000" --print=true --app=true --appPort="9999"
```

### Command-Line Flags

| Flag          | Type    | Default   | Description                                                                                                           |
|---------------|---------|-----------|-----------------------------------------------------------------------------------------------------------------------|
| `--listen`    | string  | `:8080`   | Address (port) on which the Inspectr proxy listens for incoming HTTP requests.                                        |
| `--backend`   | string  | `(empty)` | Backend service address (e.g. "http://localhost:3000"). If empty, the proxy returns a default 200 OK response.        |
| `--broadcast` | string  | `(empty)` | Optional, a remote URL to which the captured requests are sent via HTTP POST ( e.g. "http://localhost:9999/api/sse"). |
| `--print`     | boolean | `false`   | If true, prints a color‑coded summary of each request/response to the console.                                        |
| `--app`       | boolean | `false`   | If true, starts the embedded Inspectr App service (serves static assets and SSE endpoints).                           |
| `--appPort`   | string  | `4004`    | Port on which the Inspectr App service runs when `--app` is enabled.                                                  |

### Running the Embedded Inspectr UI

When the `--app` flag is enabled, the proxy starts a separate server to serve the Inspectr App UI.

For example, if you run:

```bash
./inspectr --listen=":8080" --app=true
```

Then visit http://localhost:4004 in your browser to view the Inspectr UI. The SSE endpoint is available
at http://localhost:4004/api/sse.

## How It Works

1. Proxy Mode:

   The proxy listens on the specified `--listen` address. If a backend is defined via `--backend`, it forwards the
   incoming
   HTTP request to that backend and relays the response back to the client.

2. Data Capture & Wrapping:

   All request and response details are captured into an Inspectr structure. This data is then wrapped in a
   CloudEvents envelope.

3. Embedded Inspectr App:

   The embedded Inspectr App server runs on the port specified by `--appPort` and serves the Inspectr UI for real‑time
   updates.

4. Broadcasting & Logging:

    - If the `--print` flag is enabled, a color‑coded summary of each transaction is printed to the console.
    - If a `--broadcast` URL is provided, the CloudEvent is sent via HTTP POST to that destination, like a remote setup
      of the Inspectr App on Vercel.
    - If the embedded app mode is enabled (`--app`), the data is also broadcast internally via SSE.

## Use-case examples

### Inspecting API HTTP Traffic

Suppose you have a backend service running on port `4005`. You want to inspect all HTTP request and responses sent to
the backend service, log the details to the console, and view the request and response data in real time via the
Inspectr UI running on port http://localhost:4004.

You would start the proxy as follows:

```bash
./inspectr --listen=":8080" \
--backend="http://localhost:3000" \
--print=true \
--app=true
```

Explanation:

- The proxy listens on port `8080` and forwards requests to the backend at "http://localhost:3000.
- Captured request and response data is printed to the console.
- Captured data is also sent to the Inspectr UI service at port `4004`. Visit http://localhost:4004 to view the Inspectr
  UI.

### Inspecting Webhook Events

In this case you want to inspect webhook events sent from a third-party service. You may not have a backend service to
forward the requests to—instead you simply want to capture and inspect the incoming webhook payloads. You can start the
proxy without a backend so that every incoming request receives a default 200 OK response while still capturing and
broadcasting the webhook data:

```bash
./inspectr --listen=":8080" \
--print=true \
--app=true
```

Explanation:

- The proxy listens on port 8080 and immediately returns a 200 OK response for every incoming webhook event.
- The webhook event payload details are captured and printed to the console.
- The captured webhook event is send to the Inspectr UI service. Visit http://localhost:40004 to see the Inspectr UI in
  real time.

## Local development

### Setup for local development

1. Clone the Repository:

```bash
git clone https://github.com/thim81/inspectr-proxy.git
cd inspectr-proxy
```

2. Install Go: If you don't have Go installed, download and install it from https://go.dev/dl/. Set up your GOPATH and
   GOROOT environment variables.

3. Setup Dependencies:

```bash
bash setup.sh
```

4. Build from source as binary

```bash
go build
```

5. Run the binary:

```bash
./inspectr --listen=":8080" --backend="http://localhost:4005" --print=true --app=true
curl http://localhost:8080/
```

By default the Inspectr Proxy listens on port `8080` and the Inspectr App is available on http://localhost:9999

### Build binaries

