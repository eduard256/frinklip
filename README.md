# frinklip

Local LAN drag-and-drop file server. Drop any files into the browser — they land in `/tmp/dropped/` and the page returns ready-to-paste prompts for an AI assistant, such as:

```
Посмотри файл /tmp/dropped/1713254400_report.pdf
Прочитай изображение /tmp/dropped/1713254401_photo.jpg
```

## Features

- Single Go binary, zero runtime dependencies
- Drag-and-drop any file (or multiple at once)
- Per-file upload progress bar
- Auto-copy resulting text block to clipboard
- Manual "Copy all" button
- Session history of recent uploads
- Image detection (jpg, png, gif, webp, bmp, svg) uses the "Прочитай изображение" prefix
- Runs on :80 across the whole LAN (bind `0.0.0.0`)
- systemd unit for autostart, runs as a dedicated unprivileged user with `CAP_NET_BIND_SERVICE`

## Install

```bash
git clone https://github.com/eduard256/frinklip.git
cd frinklip
make install
```

The `make install` target builds the binary, creates the `filedrop` system user, installs the systemd unit, grants `CAP_NET_BIND_SERVICE` so the binary can bind port 80 without root, and enables+starts the service.

After install open `http://<machine-ip>/` from any device in the LAN.

## Manual steps

If you want to run without systemd:

```bash
go build -o frinklip ./cmd/frinklip
sudo setcap 'cap_net_bind_service=+ep' ./frinklip
./frinklip
```

## Uninstall

```bash
sudo make uninstall
```

## License

MIT
