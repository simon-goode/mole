# mole - Mobile One-time Local Exchange

<p align="center">
  <img src="media/banner.png" alt="mole">
</p>

Lightweight peer-to-peer file transfer tool. Files stream directly from phone to computer over your local network.

## Installation

```bash
go install github.com/simongoode/mole@latest
```

Or download a prebuilt binary from the releases page.

## Usage

```
mole [-p <port>] [mode] [safe] [dir]
mole go [dir]
```

### Modes

| Command | Behaviour |
|---|---|
| `mole` | General file uploader (accepts anything) |
| `mole anything` | Same as `mole` |
| `mole photos` | Accept only image files (JPEG, PNG, GIF, WebP, etc.) |
| `mole pdfs` | Accept only PDF files |
| `mole text` | Show a text paste box instead of file upload |

### Flags

| Flag | Description |
|---|---|
| `-p`, `--port <port>` | Specify server port (default: 8080, auto-increments if busy) |

### Options

| Option | Description |
|---|---|
| `safe` | Queue files instead of saving immediately; release with `mole go` |
| `<dir>` | Save files to a custom directory (relative or absolute path) |

### Subcommands

| Command | Description |
|---|---|
| `mole go` | Release queued files to the original destination |
| `mole go <dir>` | Release queued files to a different directory |

## Examples

```bash
# General uploader, files go to ~/Downloads
mole

# Photos only, saved to ./vacation
mole photos ./vacation

# PDFs only, queue mode
mole pdfs safe

# Text mode (paste box)
mole text

# Custom port
mole -p 3000

# Queue files, then release
mole photos safe ./pics
# ... scan QR, upload photos ...
mole go

# Release queued files to a different directory
mole go ./final-edit

# All options together
mole photos safe ./pics -p 9090
```

## How it works

1. `mole` starts an HTTP server on your machine and shows a QR code in the terminal
2. Scan the QR code with your phone to open the web page
3. Select files, take a photo, or paste text — they stream directly to your computer
4. No data passes through any external server; everything stays on your LAN

## Network notes

- Works best on home/office Wi-Fi where devices can communicate directly
- Public Wi-Fi or enterprise networks may block peer-to-peer connections

## Build from source

```bash
git clone https://github.com/simongoode/mole
cd mole
go build -o mole ./src/
```
