# meetjoiner

I've found that I often miss desktop notifications for meetings even though I have several reminders set up. To address this, I've created `meetjoiner`, which watches your Google Calendar for accepted meetings and, 2 minutes after the scheduled start time, if you are not already in the meeting, opens the Meet link in Chrome, mutes mic/camera, and clicks "Join now". The 2 minute buffer provides a space in case you haven't joined due to wrapping up another meeting.


## Requirements

- **macOS** (uses AppleScript to control Chrome)
- **Google Chrome** with JavaScript from Apple Events enabled:
  - Chrome menu bar → **View** → **Developer** → **Allow JavaScript from Apple Events**
- **Google Calendar API OAuth 2.0 credentials** (Desktop app type) from a [Google Cloud project](https://console.cloud.google.com/apis/credentials) with the Google Calendar API enabled

## Quick start

```sh
# Install the binary
go install github.com/codenaugh/meetjoiner@latest

# Place your OAuth credentials
mkdir -p ~/.config/meetjoiner
cp /path/to/client_secret_XXXXX.json ~/.config/meetjoiner/credentials.json

# First run — authenticates via browser
meetjoiner -calendar you@example.com

# Install as a background service (runs on login)
meetjoiner -calendar you@example.com install
```

On first run, a browser window opens for Google OAuth consent. After you approve, the token is saved and the daemon begins polling your calendar.

## Usage

```
meetjoiner -calendar <calendar-id>            # run in foreground
meetjoiner -calendar <calendar-id> install    # install as LaunchAgent
meetjoiner uninstall                          # remove LaunchAgent
```

- `-calendar` — the Google Calendar ID to watch (typically your email address, defaults to `primary`)

## How it works

- Polls Google Calendar every 30 seconds, looking 5 minutes ahead
- Filters for events you've accepted (or organized) that have a Google Meet link
- 2 minutes after the scheduled start time, checks if you're already in the meeting (by scanning Chrome tabs)
- If not, opens the Meet link in a new Chrome tab, waits for the pre-join screen, turns off mic and camera, and clicks "Join now"

## Logs

When running as a LaunchAgent, logs go to `/tmp/meetjoiner.log`:

```sh
tail -f /tmp/meetjoiner.log
```

## Managing the LaunchAgent

```sh
# Stop
launchctl unload ~/Library/LaunchAgents/com.meetjoiner.plist

# Restart
launchctl kickstart -k gui/$(id -u)/com.meetjoiner

# Uninstall
meetjoiner uninstall
```
