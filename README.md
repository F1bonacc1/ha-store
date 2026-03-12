# ha-store

`ha-store` is a Go-based REST API service that provides an interface for the NATS Object Store. It allows users to manage files and directories through a simple HTTP API, leveraging the reliability and scalability of NATS.

## Prerequisites

- Go 1.20 or higher
- Docker and Docker Compose (for running NATS dependencies)
- TLS certificates for mTLS authentication (optional, for production)

## Getting Started

### 1. Start Dependencies

Start the NATS server using Docker Compose:

```bash
make docker-up
```

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

Build the application:

```bash
make build
```

Run the application with mTLS:

```bash
./ha-store -tls-cert=server.crt -tls-key=server.key -tls-ca=ca.crt
```

The server will start on port `8090` by default with HTTPS and mTLS enabled.

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

Stop Docker dependencies and remove build artifacts:

```bash
make docker-down
make clean
```
