# ha-store

`ha-store` is a Go-based REST API service that provides an interface for the NATS Object Store. It allows users to manage files and directories through a simple HTTP API, leveraging the reliability and scalability of NATS.

## Prerequisites

- Go 1.20 or higher
- [NATS server](https://nats.io/download/) (`nats-server` binary in PATH)
- [process-compose](https://f1bonacc1.github.io/process-compose/) (for running the local NATS cluster)
- TLS certificates for mTLS authentication (optional, for production)

## Getting Started

### 1. Start Dependencies

Start a 3-node NATS cluster using process-compose:

```bash
process-compose up
```

This launches a local 3-node NATS JetStream cluster (ports 4220–4222) along with the ha-store server. See `process-compose.yaml` for details.

### 2. Build and Run

Build the application:

```bash
make build
```

Run the application:

```bash
# Development mode (no TLS)
./ha-store

# Production mode with mTLS
./ha-store -tls-cert=server.crt -tls-key=server.key -tls-ca=ca.crt
```

The server will start on port `8090` by default.

## Configuration

The application can be configured using command-line flags or environment variables.

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `-nats-url` | `NATS_URL` | `nats://localhost:4222` | NATS server URL |
| `-port` | `PORT` | `8090` | Port for the HTTPS server |
| `-replicas` | `REPLICAS` | `1` | Number of replicas for the object store |
| `-tls-cert` | `TLS_CERT_FILE` | - | TLS certificate file path |
| `-tls-key` | `TLS_KEY_FILE` | - | TLS private key file path |
| `-tls-ca` | `TLS_CA_FILE` | - | CA certificate for client verification |
| `-throttle-speed` | `THROTTLE_SPEED` | `150` | Upload throttle speed in MB/s |
| `-upload-deadline` | `UPLOAD_DEADLINE` | `600` | Upload/download context deadline in seconds |
| `-delete-deadline` | `DELETE_DEADLINE` | `60` | Delete context deadline in seconds |

## API Documentation

All files are stored in a single shared bucket. When mTLS is enabled, client certificate authentication is required.

### Files

- **Upload a file**
  ```http
  PUT /files/<path>
  ```
  Uploads a file to the specified path using multipart/form-data.

  **Example:**
  ```bash
  curl -X PUT -F "file=@/path/to/local/file.txt" http://localhost:8090/files/documents/file.txt
  ```

- **Download a file**
  ```http
  GET /files/<path>
  ```
  Downloads the file from the specified path.

  **Example:**
  ```bash
  curl -O http://localhost:8090/files/documents/file.txt
  ```

- **Delete a file** (async)
  ```http
  DELETE /files/<path>
  ```
  Deletes the file at the specified path asynchronously. Returns `202 Accepted` immediately; the actual deletion happens in the background.

### Directories

- **Create a directory or Upload Archive**
  ```http
  PUT /dirs/<path>?extract=<type>
  ```
  Creates a directory at the specified path. If a file is attached and `extract` query parameter is provided, the file is treated as an archive and extracted into the directory.
  
  **Supported `extract` types:**
  - `zip` (.zip)
  - `tgz` or `targz` (.tar.gz)
  - `zst` (.tar.zst)
  - `7z` or `7zip` (.7z)

  **Example (Create empty directory):**
  ```bash
  curl -X PUT http://localhost:8090/dirs/documents
  ```

  **Example (Upload and extract zip):**
  ```bash
  curl -X PUT -F "file=@archive.zip" "http://localhost:8090/dirs/documents?extract=zip"
  ```
  This will extract contents of `archive.zip` into `documents/` path.

- **List directory contents**
  ```http
  GET /dirs/<path>
  ```
  Lists all files within the specified directory prefix.

  **Example:**
  ```bash
  curl http://localhost:8090/dirs/documents
  ```

- **Delete a directory**
  ```http
  DELETE /dirs/<path>
  ```
  Deletes the directory and its contents.

  **Example:**
  ```bash
  curl -X DELETE http://localhost:8090/dirs/documents
  ```

## stctl — CLI Client

`stctl` is a command-line tool to interact with an ha-store server.

### Build

```bash
make build-stctl
```

The binary is placed at `bin/stctl`.

### Usage

```bash
# Set server (default: http://localhost:8090)
export STCTL_SERVER=http://localhost:8090
# or use --server / -s flag

# Connect with mTLS
stctl --server https://localhost:8090 --tls-cert client.crt --tls-key client.key --tls-ca ca.crt <command>
```

| Flag | Default | Description |
|------|---------|-------------|
| `--server`, `-s` | `http://localhost:8090` | ha-store server URL |
| `--tls-cert` | - | Client certificate file for mTLS |
| `--tls-key` | - | Client private key file for mTLS |
| `--tls-ca` | - | CA certificate file for verifying the server |

#### Files

```bash
# Upload a file
stctl file put docs/readme.txt ./README.md

# Download a file
stctl file get docs/readme.txt ./downloaded.txt

# Download to stdout
stctl file get docs/readme.txt

# Delete a file
stctl file rm docs/readme.txt
```

#### Listing

```bash
# List directory contents
stctl ls mydir

# Long listing format (permissions, owner, size, mod time)
stctl ls -l mydir
```

#### Directories

```bash
# Create an empty directory
stctl dir create mydir

# Upload and extract an archive (default type: tgz)
stctl dir extract mydir ./archive.tgz

# Specify archive type (zip, tgz, targz, zst, 7z, 7zip)
stctl dir extract mydir ./archive.zip --type zip

# Delete a directory and its contents
stctl dir rm mydir
```

## Development

### Running Tests

Run unit and integration tests:

```bash
make test
```

### Test Coverage

Generate and view test coverage report:

```bash
make coverage
```

### Clean Up

Stop the local cluster and remove build artifacts:

```bash
# Stop process-compose (Ctrl+C in the terminal, or)
process-compose down

# Remove build artifacts
make clean
```
