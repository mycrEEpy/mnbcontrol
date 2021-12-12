# Midnight Brawlers Control

`mnbcontrol` is the API & Daemon powering the Midnight Brawlers
server landscape.

## Configuration

In order for `mnbcontrol` to function properly you must configure some
required environment variables and pass some flags.

### Environment Variables 

| Variable          | Description                                                                   |
|-------------------|-------------------------------------------------------------------------------|
| HCLOUD_TOKEN      | Token for interacting with the Hetzner Cloud API (read/write access required) |
| HCLOUD_DNS_TOKEN  | Token for interacting with the Hetzner DNS API                                |
| DISCORD_KEY       | Discord Access Token used for OAuth2                                          |
| DISCORD_SECRET    | Discord Secret Token used for OAuth2                                          |
| DISCORD_BOT_TOKEN | Discord Bot Token for interacting with the Discord API                        |
| JWT_SIGNING_KEY   | Secret for singing the JSON Web Tokens                                        |

### Flags

| Flag                   | Type   | Default                                              | Description                                         |
|------------------------|--------|------------------------------------------------------|-----------------------------------------------------|
| logLevel               | int    | 4                                                    | log level (0-6)                                     |
| logReportCaller        | bool   | true                                                 | log report caller                                   |
| logFormatterJson       | bool   | false                                                | log formatter json                                  |
| listenAddr             | string | :8000                                                | http server listen address                          |
| locationName           | string | nbg1                                                 | Hetzner location name                               |
| networkIDs             | string |                                                      | comma separated list of network ids                 |
| sshKeyIDs              | string |                                                      | comma separated list if ssh key ids                 |
| dnsZoneID              | string |                                                      | dns zone id, can be empty for disabling dns support |
| discordCallback        | string | http://localhost:8000/auth/callback?provider=discord | discord oauth callback url                          |
| discordGuildID         | string |                                                      | discord guild id for authorization                  |
| discordChannelID       | string |                                                      | discord channel id for user interaction             |
| discordAdminRoleID     | string |                                                      | discord role id for admin authorization             |
| discordUserRoleID      | string |                                                      | discord role id for user authorization              |
| discordPowerUserRoleID | string |                                                      | discord role id for power user authorization        |
