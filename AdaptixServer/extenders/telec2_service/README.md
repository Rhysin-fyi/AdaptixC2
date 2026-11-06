# TeleC2 Adaptix service

This local service adds a native TeleC2 connections panel to Adaptix. It shows
configured Linux, Windows, and macOS destinations in a connection table, keeps
a separate console transcript for each destination, verifies Telegram access,
requests TeleC2 status, and sends an operator-selected file through a dedicated
Telegram operator bot. It does not execute uploaded files or expose TeleC2's
`/run` command.

## Telegram setup

1. Create a dedicated operator bot with BotFather.
2. Enable Bot-to-Bot Communication Mode for the operator and TeleC2 bots.
3. Put both bots in a private group. Make the TeleC2 bot an administrator and
   disable its Group Privacy Mode so it receives document messages.
4. Restrict TeleC2 to the private group with `ALLOWED_CHAT_IDS` and authorize
   the operator bot's numeric ID with `ALLOWED_USER_IDS`.
5. Do not use `ALLOWED_USER_IDS=*`.

The operator bot should be used only by this service. Sharing its update stream
with another process will cause missed replies.

## Adaptix setup

The Fedora installer creates a private configuration placeholder automatically.
Edit it after installation:

```bash
$EDITOR install/server/extenders/telec2_service/telec2-config.json
```

For a non-Fedora/manual installation, copy `telec2-config.example.json` to
`telec2-config.json` beside the service plugin and run `chmod 600` on it.

Edit `telec2-config.json`, then add this extender config to the server profile:

```yaml
extenders:
  - "extenders/telec2_service/config.yaml"
```

Restart the server and client, then open the **TeleC2** dock:

- **Connections** shows platform, bot identity, connection state, and the last
  real TeleC2 reply time.
- **Console** keeps status and reply history isolated per destination.
- **Transfer** selects and uploads a file after an explicit confirmation.
- **Health & Setup** explains the selected connection and verifies the operator
  bot token and Telegram group access without returning the token to the client.

Bot creation, Bot-to-Bot Communication Mode, and Group Privacy Mode remain
one-time BotFather administration. Routine verification, status, transfers,
and replies are handled inside Adaptix.

The Telegram hosted Bot API limits files downloaded by TeleC2 to 20 MiB. The
service verifies the configured limit before sending and reports the SHA-256
digest after Telegram accepts the document.

Keep `telec2-config.json` private. It is intentionally excluded from build
artifacts and must never be committed.
