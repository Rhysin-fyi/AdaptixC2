# TeleC2 Adaptix service

This local service adds a TeleC2 transfer panel to Adaptix. It can request
TeleC2 status and send an operator-selected file through a dedicated Telegram
operator bot. It does not execute uploaded files or expose TeleC2's `/run`
command.

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

After building and installing the extenders:

```bash
cd install/server/extenders/telec2_service
cp telec2-config.example.json telec2-config.json
chmod 600 telec2-config.json
```

Edit `telec2-config.json`, then add this extender config to the server profile:

```yaml
extenders:
  - "extenders/telec2_service/config.yaml"
```

Restart the server and client. Open the **TeleC2** dock, select a destination,
choose a file, review the size confirmation, and click **Upload**.

The Telegram hosted Bot API limits files downloaded by TeleC2 to 20 MiB. The
service verifies the configured limit before sending and reports the SHA-256
digest after Telegram accepts the document.

Keep `telec2-config.json` private. It is intentionally excluded from build
artifacts and must never be committed.
