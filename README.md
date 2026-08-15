# ClipBox

[English](README.md) | [简体中文](README_CN.md)

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8.svg)](https://go.dev/)
[![Gin](https://img.shields.io/badge/Gin-1.10-008ECF.svg)](https://gin-gonic.com/)
[![GitHub license](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

ClipBox is a small temporary sharing service for files, text, and links. A five-digit pickup code resolves to the content while stable SHA1 paths make files and text directly addressable.

## Features

- File pickup codes redirect to `/file/<file-sha1>/<original-filename>`; that path is also directly downloadable.
- Text pickup codes redirect to `/text/<text-sha1>`.
- Link pickup codes keep the original behavior and redirect straight to the destination URL.
- Files are uploaded in parallel chunks. Incomplete chunks are kept under `data/tmp/<sha1>` and can be resumed for 10 minutes.
- The server verifies the complete file SHA1 before moving it into `data/files/<sha1>`.
- Database access uses GORM. File metadata lives in a separate `cb_files` table linked from `cb_clips.file_id`.
- New content defaults to 1,000 accesses and expires after one day. The server automatically removes expired records and unreferenced files every minute.
- Existing `cb_clips` tables are migrated automatically, including legacy file records and missing SHA1 values.

## Stack

- Backend: Go, Gin, GORM, SQLite/MySQL
- Frontend: Vue 3, Vite
- Storage: local content-addressed files under `data/`

## Screenshots

### Pickup and sending

<p align="center">
  <img src=".github/images/img1.png" alt="ClipBox pickup home" width="100%">
</p>

<p align="center">
  <img src=".github/images/img2.png" alt="ClipBox send file" width="100%">
</p>

### File details and sending history

<p align="center">
  <img src=".github/images/img3.png" alt="ClipBox file details" width="100%">
</p>

<p align="center">
  <img src=".github/images/img4.png" alt="ClipBox sending history" width="100%">
</p>

## Quick start

Requirements: Go 1.23+ and Node.js 20+. SQLite is the default, so no database server is required.

```bash
cd frontend
npm ci
npm run build
cd ..
go run .
```

The first start creates `data/config.json` and the SQLite database at `data/clipbox.db`. SQLite uses a pure-Go driver, so the release binaries also work with `CGO_ENABLED=0`. ClipBox listens on `http://127.0.0.1:5328` by default. For frontend development, run `npm run dev` in `frontend/`; Vite proxies `/clip`, `/file`, and `/text` to the Go server.

## Configuration

Configuration is stored in `data/config.json`. The default database settings are:

```json
{
  "database": {
    "driver": "sqlite",
    "dsn": "data/clipbox.db"
  }
}
```

To use MySQL, change `driver` to `mysql`, set `dsn` to a Go MySQL DSN such as `root:password@tcp(127.0.0.1:3306)/clipbox?charset=utf8mb4`, and restart the service. The complete configuration and defaults are written on first start; see [.env.example](.env.example) for optional environment overrides.

## Upload API

1. `POST /clip/upload/init` with JSON `{filename,size,sha1,count,expire}`. The response includes the server chunk size, worker count, and the indexes already uploaded.
2. Upload missing raw chunks concurrently with `PUT /clip/upload/<sha1>/<chunk-index>`.
3. `POST /clip/upload/<sha1>/complete` with JSON `{filename,count,expire}`.

The default chunk size is 4 MiB, concurrency is 4, and the resume window is 10 minutes. These can be changed using the environment variables documented in [.env.example](.env.example).

## Releases

The repository includes a manually triggered GitHub Actions workflow at `.github/workflows/release.yml`. Run **Build and Release** from the Actions tab to build Linux, Windows, and macOS binaries for amd64 and arm64. Each archive includes the binary, the built frontend `www/` directory, and the project documentation. The workflow publishes a release named with the Shanghai date in `YYMMDD` format and attaches a `SHA256SUMS` file.

## Test

```bash
go test ./...
cd frontend && npm run build
```

## License

[MIT](LICENSE)
