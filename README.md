# Go Media Server

A simple HTTP media server written in Go that allows browsing and streaming media files through a web interface.

## Features

- Browse directories and files in the media folder
- Stream media files (videos, audio, images) directly in the browser
- Responsive web interface
- Support for various media formats
- Ability to use a custom media directory

## Usage

### Default Mode

By default, the media server will serve files from the `./media` directory relative to where the server is run:

```bash
go run main.go
```

### Custom Media Directory

You can specify a custom media directory using the `MEDIA_DIR` environment variable:

```bash
# On Windows
set MEDIA_DIR=C:\path\to\your\media\folder
go run main.go

# On Linux/Mac
MEDIA_DIR=/path/to/your/media/folder go run main.go
```

### Building the Application

To build an executable:

```bash
go build -o media-server
```

Then run the executable:

```bash
./media-server
```

## Supported Media Formats

The server supports many common media formats:

### Video
- MP4, MKV, AVI, MOV, WMV, FLV, WebM

### Audio
- MP3, WAV, AAC, OGG, FLAC

### Images
- JPG/JPEG, PNG, GIF, BMP, WebP

## Accessing the Server

Once the server is running, access it through your web browser at:

```
http://localhost:8080
```
