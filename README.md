# ghtop

A terminal UI for monitoring GitHub Actions workflow runs. Pin specific runs across any number of repos and watch their status update live.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and the [gh CLI](https://cli.github.com/).

```
ghtop  GitHub Actions Monitor
────────────────────────────────────────────────────────────────────

 ► ✅  owner/repo  #42
        Deploy to production
        main · ✓ success · 3m ago

   ⏳  another-org/api  #1337
        Run integration tests
        feat/auth · ⏳ queued

   🔄  my-org/frontend  #99
        Build & release
        main · running · just now

────────────────────────────────────────────────────────────────────
  a add  d delete  r refresh all  R refresh  o open  j/k ↑/↓  q quit
```

## Requirements

- Go 1.21+
- [gh CLI](https://cli.github.com/) authenticated (`gh auth login`)

## Install

```sh
go install github.com/do4k/ghtop@latest
```

Or build from source:

```sh
git clone https://github.com/do4k/ghtop
cd ghtop
go build -o ghtop .
```

## Usage

Just run `ghtop`. Your pinned runs are saved to `~/.config/ghtop/pins.json` and restored on next launch.

### Key bindings

| Key | Action |
|-----|--------|
| `a` | Add a run (paste URL or `owner/repo/actions/runs/ID`) |
| `d` | Delete the selected run |
| `r` | Refresh all runs |
| `R` | Refresh selected run |
| `o` / `Enter` | Open selected run in browser |
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `q` / `Ctrl+C` | Quit |

### Adding a run

Press `a` and paste either a full URL or a short path:

```
https://github.com/owner/repo/actions/runs/12345678
owner/repo/actions/runs/12345678
```

GitHub Enterprise is supported — just paste the full URL and the hostname is extracted automatically. If you have a GHE host configured in `~/.config/gh/hosts.yml`, short-form inputs will default to that host.

## Auto-refresh

Pinned runs refresh automatically every 30 seconds.
