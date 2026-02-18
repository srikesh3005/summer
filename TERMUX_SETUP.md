# Summer on Android (Termux) - Setup Guide

This guide installs and runs Summer on an Android phone using Termux, then integrates Telegram.

## 1. Install Termux

1. Install **Termux** from F-Droid (recommended).
2. Open Termux and run:

```bash
pkg update -y && pkg upgrade -y
pkg install -y git golang jq curl openssl ca-certificates
```

Optional but recommended:

```bash
pkg install -y tmux
termux-wake-lock
```

`termux-wake-lock` helps avoid Android killing long-running sessions.

## 2. Clone and build Summer

```bash
cd ~
git clone https://github.com/srikesh3005/summer.git
cd summer
go build -o ./build/summer ./cmd/summer
./build/summer version
```

If `go build` is too slow on your device, you can cross-compile on desktop and copy the binary to Termux.

## 3. Initialize Summer

```bash
./build/summer onboard
```

This creates:

- `~/.summer/config.json`
- `~/.summer/workspace/`

## 4. Configure provider + Telegram

Edit config:

```bash
nano ~/.summer/config.json
```

Set at least:

- `agents.defaults.provider` (example: `groq`)
- provider API key under `providers.<provider>.api_key`
- `channels.telegram.enabled = true`
- `channels.telegram.token = "<YOUR_BOT_TOKEN>"`
- `channels.telegram.allow_from = ["<YOUR_TELEGRAM_USER_ID>"]`

Example minimal section:

```json
{
  "agents": {
    "defaults": {
      "provider": "groq",
      "model": "llama-3.3-70b-versatile",
      "workspace": "~/.summer/workspace",
      "restrict_to_workspace": true
    }
  },
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "123456:ABCDEF...",
      "allow_from": ["5974537769"]
    }
  },
  "providers": {
    "groq": {
      "api_key": "gsk_..."
    }
  }
}
```

## 5. Run Summer gateway

```bash
cd ~/summer
./build/summer gateway
```

If started correctly, you should see `Gateway started` and Telegram channel enabled.

## 6. Test from Telegram

Message your bot:

1. `hello`
2. `create a markdown summary and send me the file`
3. `remind me in 2 minutes to drink water`

## 7. Keep it running in background (mobile-friendly)

Use `tmux`:

```bash
tmux new -s summer
cd ~/summer
./build/summer gateway
```

Detach: `Ctrl+b` then `d`  
Reattach later:

```bash
tmux attach -t summer
```

## 8. Common fixes

### A) Bot does not reply

- Confirm `allow_from` contains your Telegram numeric user ID.
- Confirm gateway is still running.
- Check token correctness.

### B) No internet/API failures

- Verify mobile data/Wi-Fi.
- If needed, set `channels.telegram.proxy` or provider proxy in config.

### C) Config not applied

- Restart gateway after any config change.

## 9. Optional: simple startup script

Create `~/summer/start.sh`:

```bash
#!/data/data/com.termux/files/usr/bin/bash
cd ~/summer
./build/summer gateway
```

Make executable:

```bash
chmod +x ~/summer/start.sh
```

Then start with:

```bash
tmux new -s summer '~/summer/start.sh'
```

---

If you want, the next step is adding Termux auto-start on boot using the Termux:Boot app and a boot script.
