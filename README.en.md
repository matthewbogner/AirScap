<h2 align="center">AirScap</h2>

<h3 align="center">AirScan bridge for legacy Wi-Fi ScanSnap scanners</h3>

<p align="center">
  <a href="https://github.com/mzyy94/airscap/actions/workflows/test.yml"><img src="https://github.com/mzyy94/airscap/actions/workflows/test.yml/badge.svg?branch=master" alt="CI"></a>
  <a href="https://github.com/mzyy94/airscap/releases"><img src="https://img.shields.io/github/v/release/mzyy94/airscap" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/mzyy94/airscap" alt="License"></a>
</p>

<p align="center">
  <a href="#installation">Installation</a>&ensp;&bull;&ensp;
  <a href="#quick-start">Quick Start</a>&ensp;&bull;&ensp;
  <a href="#web-ui">Web UI</a>&ensp;&bull;&ensp;
  <a href="#configuration">Configuration</a>&ensp;&bull;&ensp;
  <a href="#ocr-tool">OCR</a>&ensp;&bull;&ensp;
  <a href="protocol.en.md">Protocol Docs</a>&ensp;&bull;&ensp;
  <a href="README.md">日本語</a>
</p>

---

## What is AirScap?

The **ScanSnap Connect Application** that enabled Wi-Fi scanning for the iX500 has been discontinued. The currently available ScanSnap Home does not support the iX500, leaving no way to scan over Wi-Fi. AirScap is a protocol bridge that brings legacy ScanSnap scanners including the iX500 back to life by making them AirScan-compatible.

It natively implements the proprietary **VENS** protocol used by ScanSnap iX500 over Wi-Fi in Go, and exposes it as a standard **eSCL (AirScan)** network scanner.

| Platform | Client |
|----------|--------|
| **macOS** | Image Capture, Preview |
| **Linux** | SANE (`sane-airscan`), scanservjs |
| **Windows** | Windows Scan (WSD) |

## Architecture

```mermaid
flowchart TB
    A[macOS Image Capture] & B[Linux SANE] & C[Windows Scan]

    subgraph airscap["AirScap"]
        E[eSCL Server] --- F[mDNS _uscan] --- G["Web UI (:8080)"]
    end

    A & B & C -- "eSCL (HTTP)" --> airscap
    airscap -- "VENS (UDP/TCP)" <--> S[ScanSnap iX500]
```

## Features

- **Driver-free scanning** &mdash; Works with any eSCL/AirScan client out of the box
- **Zero configuration** &mdash; Auto-discovers ScanSnap on the network and connects
- **Versatile scanning** &mdash; Color / grayscale / B&W, duplex, PDF / JPEG / TIFF output, JPEG quality control, blank page removal, bleed-through reduction
- **Physical button support** &mdash; Press the scanner button to trigger a scan job. Save to local folder / FTP / [Paperless-ngx] from your choice
- **Web UI** &mdash; Configure settings and monitor status from your browser (English / Japanese)
- **Single binary** &mdash; Pure Go, no CGO required, cross-compilable. Ships with a systemd service unit
- **OCR support** &mdash; Built-in Japanese OCR engine generates searchable PDFs directly during scanning. Also includes a standalone `ndlocr-pdf` command

[Paperless-ngx]: https://github.com/paperless-ngx/paperless-ngx

## Tested Environment

Development and testing were done with a **ScanSnap iX500** and **macOS Image Capture**. Other ScanSnap models using the same VENS protocol may work but have not been tested. Compatibility with other client software (SANE, Windows Scan, etc.) is not guaranteed.

> [!TIP]
> **Using USB on Linux?** The standard SANE driver [`sane-epjitsu`](http://www.sane-project.org/man/sane-epjitsu.5.html) supports the ScanSnap iX500 over USB. AirScap is designed for Wi-Fi scanning only &mdash; if you have a USB connection, use the SANE driver instead.

## Installation

### From Release (Debian/Ubuntu)

Download the `.deb` package from the [Releases](https://github.com/mzyy94/airscap/releases) page:

```bash
# Download the latest release
curl -LO https://github.com/mzyy94/airscap/releases/latest/download/airscap_amd64.deb

# Install
sudo dpkg -i airscap_amd64.deb

# Edit configuration
sudo vi /etc/airscap/env

# Start the service
sudo systemctl enable --now airscap
```

The deb package includes the binary, systemd service unit, and default configuration.

### From Source

Requires Go 1.25+.

```bash
git clone https://github.com/mzyy94/airscap.git
cd airscap
go mod vendor
CGO_ENABLED=0 go build -mod=vendor -overlay=build/overlay.json -o airscap ./cmd/airscap/
```

> [!NOTE]
> If CGO is available (libjpeg / libpng installed), you can build without vendor or overlay:
> ```bash
> go build -o airscap ./cmd/airscap/
> ```

### systemd (manual)

```bash
sudo cp airscap /usr/local/bin/
sudo cp dist/airscap.service /etc/systemd/system/
sudo mkdir -p /etc/airscap
sudo cp dist/env.example /etc/airscap/env
sudo vi /etc/airscap/env

sudo systemctl daemon-reload
sudo systemctl enable --now airscap
```

## Quick Start

```bash
# Auto-discover scanner, auto-derive password from serial number
./airscap

# Or specify password explicitly
AIRSCAP_PASSWORD=0700 ./airscap

# Specify scanner IP directly
AIRSCAP_PASSWORD=0700 AIRSCAP_SCANNER_IP=192.168.1.100 ./airscap
```

Once running, AirScap will:

1. Discover your ScanSnap on the local network (or connect to the specified IP)
2. Pair using the scanner password
3. Start an eSCL server on port 8080
4. Register via mDNS so clients discover it automatically
5. Serve a Web UI at `http://localhost:8080/ui/`

> [!IMPORTANT]
> AirScap cannot perform the initial Wi-Fi setup of the scanner (connecting it to an access point).
> Use the ScanSnap setup tool or the WPS button to connect the scanner to your network beforehand.

## Web UI

Access the built-in management interface at `http://<host>:8080/ui/`.

- **Status** &mdash; Connection state, ADF paper presence, error states (paper jam, cover open, multi-feed)
- **Device Info** &mdash; Scanner name, serial, IP, firmware revision
- **Button Scan Settings** &mdash; Color mode, resolution, paper size, output format, JPEG quality, duplex, blank page removal, bleed-through reduction
- **Save Destination** &mdash; Configure local folder / FTP / Paperless-ngx for button scans
- **AirScan Settings** &mdash; Auto paper size detect, bleed-through reduction, B&W density overrides for AirScan clients
- **OCR Settings** &mdash; Toggle OCR on/off for eSCL and button scans independently (shown when OCR engine is available)
- **eSCL Endpoint** &mdash; URL for manual eSCL client configuration
- **i18n** &mdash; English / Japanese toggle

## Configuration

Scanner discovery and startup settings are configured via environment variables.

| Variable | Default | Description | Notes |
|---|---|---|---|
| `AIRSCAP_PASSWORD` | auto-derive | Scanner pairing password | \* |
| `AIRSCAP_PASSWORD_FILE` | &mdash; | Path to password file | \* |
| `AIRSCAP_SCANNER_IP` | auto-discover | Scanner IP address | |
| `AIRSCAP_LISTEN_PORT` | `8080` | HTTP listen port | |
| `AIRSCAP_DEVICE_NAME` | from scanner | mDNS display name | |
| `AIRSCAP_LOG_LEVEL` | `info` | Log level (`debug` / `info` / `warn` / `error`) | |
| `AIRSCAP_ROTATE_180` | `false` | Rotate every scanned page 180°, including both duplex sides | |
| `AIRSCAP_DATA_DIR` | no persistence | Directory for persistent settings | \*\* |
| `AIRSCAP_OCR_MODEL_DIR` | &mdash; | Path to OCR model directory | \*\*\* |
| `AIRSCAP_OCR_DEVICE` | `cpu` | OCR inference device (`cpu` / `cuda`) | |

\* If you have changed the default password, specify the password you set. Use one or the other.
\*\* When running under systemd, settings are persisted to `STATE_DIRECTORY` even if unset.
\*\*\* When set, the OCR engine is enabled and can add transparent text layers to PDF output. Requires ONNX Runtime and ndlocr-lite model files.

See [`dist/env.example`](dist/env.example) for a full template.

### Scan orientation

If pages are upside down when loaded as for the official ScanSnap software, start
AirScap with `AIRSCAP_ROTATE_180=true`. This is a fixed correction, not automatic
orientation detection. It applies before eSCL/AirScan output, Web UI preview, PDF
generation (including OCR), and button/save jobs (local, FTP, and Paperless-ngx).
Page order, front/back identifiers, pixel dimensions, and resolution are unchanged.
Restart AirScap after changing the variable; it is not a Web UI setting.

Accepted true values: `1`, `t`, `T`, `true`, `TRUE`, `True`; false values:
`0`, `f`, `F`, `false`, `FALSE`, `False`. Unset/empty defaults to disabled;
invalid values produce a warning and use the default. When disabled, image bytes
pass through unchanged, without image decoding or re-encoding.

Rotation is performed one page at a time, so streaming scans do not buffer the
whole document. JPEGs are decoded, rotated without cropping, and re-encoded at
quality 95 using Go's JPEG codec. **JPEG recompression is lossy**, may change file
size, and overrides the original JPEG compression level for the resulting file.
JFIF density and other JPEG APP/comment segments are retained. Embedded thumbnails,
if present, are not regenerated. A coefficient-domain lossless JPEG transform is
not provided by the existing codecs; adding one would require another dependency
or executable, and partial MCU blocks need special edge handling.

B&W TIFFs are rotated without pixel loss and re-encoded as 8-bit grayscale Deflate
TIFF rather than CCITT G4. This may increase file size; clients must support Deflate
TIFF. X/Y resolution and resolution units are retained; other auxiliary TIFF tags
are not copied. Invalid image data fails the scan explicitly. Empty pages from
scanner-side blank-page removal remain empty.

#### Build and verify alongside an existing binary

These commands leave an existing `./airscap` and system installation untouched:

```bash
CGO_ENABLED=0 go test -mod=vendor -overlay=build/overlay.json ./...
CGO_ENABLED=0 go vet -mod=vendor -overlay=build/overlay.json ./...
CGO_ENABLED=0 go build -mod=vendor -overlay=build/overlay.json -o ./airscap-rotate180 ./cmd/airscap/
AIRSCAP_ROTATE_180=true AIRSCAP_SCANNER_IP=192.168.1.100 AIRSCAP_LISTEN_PORT=8081 AIRSCAP_DEVICE_NAME='ScanSnap rotation test' ./airscap-rotate180
```

Use your scanner IP and pairing credentials as needed. Do not connect the original
and test AirScap processes (or the macOS ScanSnap software) to the scanner at the
same time. An alternate HTTP port does not isolate scanner sessions or the button
listener. Arrange for the existing process to be inactive before testing, without
replacing its binary or editing its service configuration. To compare original
behavior using the test binary, unset the variable or set it to `false`.

Automated tests use synthetic images and a local fake scanner, not an iX500.
Manual verification with the actual scanner is still required:

1. Load two numbered, asymmetric sheets in the usual macOS orientation, marking
   the top and front/back. Compare rotation disabled versus enabled.
2. Check Web UI Scan & Preview and Skanpage/eSCL in simplex and duplex: all sides
   upright, correct page order/count, unchanged dimensions and physical PDF size.
3. Check color/grayscale JPEG, B&W TIFF, and PDF output at auto and fixed DPI.
   Verify B&W TIFF opens in the intended client; compare fine text/image quality.
4. Test button saves and configured FTP/Paperless-ngx destinations; if OCR is
   enabled, check searchable PDF text aligns with the rotated image.
5. Test a longer duplex stack, blank-page removal, and stopping an eSCL scan early
   to confirm feeding/cancellation behavior remains unchanged.

## Verify

```bash
# Check mDNS advertisement (macOS)
dns-sd -B _uscan._tcp

# Check eSCL capabilities
curl -s http://localhost:8080/eSCL/ScannerCapabilities | head -20

# Scan via SANE
scanimage -L
scanimage --device 'airscan:e0:ScanSnap iX500' --format=jpeg -o scan.jpg
```

## OCR Tool

AirScap includes a built-in Japanese OCR engine, a Go reimplementation of [NDLOCR-Lite](https://github.com/ndl-lab/ndlocr-lite), using ONNX Runtime for layout detection (DEIM) and character recognition (PARSeq).

### Integrated OCR

Set `AIRSCAP_OCR_MODEL_DIR` to enable the OCR engine. You can toggle OCR on/off for eSCL scans and button scans independently from the Web UI. When enabled, PDF output automatically includes a transparent text layer for searchable PDFs.

```bash
# Start with OCR enabled
AIRSCAP_OCR_MODEL_DIR=./ndlocr-lite/src ./airscap
```

If OCR processing fails for a page, it falls back to a plain image PDF with a warning log.

### Standalone `ndlocr-pdf` Command

The bundled `ndlocr-pdf` command applies Japanese OCR to scanned images and generates searchable PDFs with selectable text.

### Prerequisites

- [ONNX Runtime](https://onnxruntime.ai/) shared library (`libonnxruntime.dylib` / `libonnxruntime.so`)
- ndlocr-lite model files (included as a submodule)

```bash
# macOS
brew install onnxruntime

# Linux (Ubuntu/Debian)
apt install libonnx-dev

# Fetch submodule
git submodule update --init
```

### Build & Usage

```bash
# Build
go build -o ndlocr-pdf ./cmd/ndlocr-pdf/

# Single image → searchable PDF
./ndlocr-pdf --sourceimg scan.jpg --output output.pdf

# All images in a directory → single PDF
./ndlocr-pdf --sourcedir ./scanned/ --output book.pdf

# Also output JSON/TXT
./ndlocr-pdf --sourcedir ./scanned/ --output book.pdf --json --txt
```

| Flag | Default | Description |
|------|---------|-------------|
| `--sourceimg` | | Input image file |
| `--sourcedir` | | Input image directory |
| `--output` | (required) | Output PDF path |
| `--device` | `cpu` | Inference device (`cpu` / `cuda`) |
| `--modeldir` | `ndlocr-lite/src` | Model directory |
| `--json` | `false` | Output JSON per image |
| `--txt` | `false` | Output text per image |

Supported image formats: JPEG, PNG, BMP, TIFF, JPEG 2000

### Library Usage

The `ndlocr` package can also be used as a Go library.

```go
import "github.com/mzyy94/airscap/ndlocr"

engine, err := ndlocr.NewEngine(ndlocr.Config{
    ModelDir:    "ndlocr-lite/src",
    RuntimePath: "/opt/homebrew/lib/libonnxruntime.dylib",
    Device:      "cpu",
})
defer engine.Close()

blocks, err := engine.Recognize(ctx, img)
for _, b := range blocks {
    fmt.Printf("%s (%.2f)\n", b.Text, b.Confidence)
}
```

## Protocol

AirScap implements the **VENS** protocol &mdash; a proprietary binary protocol used by Fujitsu/Ricoh ScanSnap scanners over Wi-Fi. The implementation is based on analysis of packet captures from the official application.

Full protocol documentation:
- [protocol.md](protocol.md) (Japanese)
- [protocol.en.md](protocol.en.md) (English)

## License

[MIT](LICENSE)

### Third-party Licenses

This software uses the following open-source libraries:

| Library | License |
|---|---|
| [go-mfp](https://github.com/OpenPrinting/go-mfp) | BSD 2-Clause |
| [go-pdf/fpdf](https://codeberg.org/go-pdf/fpdf) | MIT |
| [zeroconf](https://github.com/grandcat/zeroconf) | MIT |
| [jlaffaye/ftp](https://github.com/jlaffaye/ftp) | ISC |
| [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) | BSD 3-Clause |
| [onnxruntime-purego](https://github.com/shota3506/onnxruntime-purego) | Apache 2.0 |
| [yaml.v3](https://github.com/go-yaml/yaml) | MIT / Apache 2.0 |
| [Noto Sans JP](https://fonts.google.com/noto/specimen/Noto+Sans+JP) (metrics-only embed) | OFL 1.1 |
| [Alpine.js](https://alpinejs.dev/) | MIT |
| [Bulma](https://bulma.io/) | MIT |
| [Feather Icons](https://feathericons.com/) | MIT |
