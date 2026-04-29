# frinklip

Local file drop server. Drag files into the browser, get absolute paths, paste into your CLI agent.

![drop files, get paths your agent can read](docs/screenshots/hero.webp)

## What it solves

CLI agents like `claude-code`, `opencode`, `codex` read files by path. SSH terminals don't accept drag-n-drop. `scp` for every screenshot is the loop you don't want.

Drop files in the browser from any LAN device. They land in `/tmp/dropped/`. Paste the absolute path into the agent.

![paste-ready paths and recent drops](docs/screenshots/paths.webp)

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/eduard256/frinklip/main/install.sh | sudo bash
```

Open `http://<lan-ip>:3467` from any device on the network.

## Manual install

Don't want to pipe a script into shell? See [INSTALL.md](INSTALL.md).

## License

MIT
