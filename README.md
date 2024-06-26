# Midnight Brawlers Control

[![Go Report Card](https://goreportcard.com/badge/github.com/mycreepy/mnbcontrol)](https://goreportcard.com/report/github.com/mycreepy/mnbcontrol)
[![Known Vulnerabilities](https://snyk.io/test/github/mycrEEpy/mnbcontrol/badge.svg)](https://snyk.io/test/github/mycrEEpy/mnbcontrol)

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
| JWT_SIGNING_KEY   | Secret for signing the JSON Web Tokens                                        |

### Flags

| Flag                   | Type   | Default                                              | Description                                         |
|------------------------|--------|------------------------------------------------------|-----------------------------------------------------|
| logLevel               | int    | 4                                                    | log level (0-6)                                     |
| logReportCaller        | bool   | true                                                 | log report caller                                   |
| logFormatterJson       | bool   | false                                                | log formatter json                                  |
| listenAddr             | string | :8000                                                | http server listen address                          |
| locationName           | string | nbg1                                                 | Hetzner location name                               |
| networkIDs             | string |                                                      | comma separated list of network ids                 |
| sshKeyIDs              | string |                                                      | comma separated list of ssh key ids                 |
| dnsZoneID              | string |                                                      | dns zone id, can be empty for disabling dns support |
| discordCallback        | string | http://localhost:8000/auth/callback?provider=discord | discord oauth callback url                          |
| discordGuildID         | string |                                                      | discord guild id for authorization                  |
| discordChannelID       | string |                                                      | discord channel id for user interaction             |
| discordAdminRoleID     | string |                                                      | discord role id for admin authorization             |
| discordUserRoleID      | string |                                                      | discord role id for user authorization              |
| discordPowerUserRoleID | string |                                                      | discord role id for power user authorization        |

## License

Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
