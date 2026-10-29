# meetjoiner

`meetjoiner` watches your macOS Calendar for accepted meetings and, right at the scheduled start time, if you are not already in the meeting, displays a floating macOS HUD card centered on your current virtual desktop.

From the HUD card you can toggle **Microphone** and **Camera** (remembering your last settings, or defaulting to both off), **Snooze 2m**, or click **Join Meeting** to open Chrome, apply your mic/camera state on the pre-join screen, and join automatically.

## Requirements

1. **macOS Calendar**: Add your Google account under **System Settings → Internet Accounts** (with **Calendars** enabled so your meetings sync to the built-in macOS Calendar app).
2. **Google Chrome** with **Allow JavaScript from Apple Events** enabled (required so `meetjoiner` can toggle your mic/camera and click "Join now" on the Meet pre-join screen):
   - In the Chrome menu bar, go to **View** → **Developer** → **Allow JavaScript from Apple Events** and make sure it is checked.
3. Only works with **Google Meet**.

## Quick start

1. In Chrome, enable **View → Developer → Allow JavaScript from Apple Events**.
2. Clone, build, and install the background service:

```sh
git clone https://github.com/codenaugh/meetjoiner.git
cd meetjoiner

# 1. Compile the Go source and put the `meetjoiner` CLI binary in ~/go/bin
go install

# 2. Package MeetJoiner.app, grant Calendar access, and start the background LaunchAgent on login
meetjoiner install
```

Or to watch a specific calendar/account only:

```sh
# List your synced macOS calendars
meetjoiner calendars

# Install watching only a specific calendar or account
meetjoiner -calendar you@example.com install
```

To preview the HUD pop-up at any time:

```sh
meetjoiner test-popup
```

## Usage

```
meetjoiner install                            # grant Calendar access and install as LaunchAgent
meetjoiner -calendar <name-or-email> install  # watch only matching calendar/account
meetjoiner calendars                          # list synced macOS calendars
meetjoiner test-popup                         # preview the HUD & save default audio/video prefs
meetjoiner                                    # run in foreground
meetjoiner uninstall                          # remove LaunchAgent
```

- `-calendar` — optional filter matching the calendar title or account name in macOS Calendar (defaults to `all`)

## How it works

- Queries macOS `EventKit` every 30 seconds, looking 5 minutes ahead (zero Google Cloud / OAuth setup required)
- Filters for events you've accepted (or organized) that contain a Google Meet link
- Schedules a timer for the exact start time of the meeting
- At the start time, checks if you're already in the meeting (by scanning Chrome tabs)
- If not, pops up a native frosted-glass HUD overlaying your active virtual desktop with:
  - Live elapsed time indicator & attendee summary
  - **Microphone (`M`)** and **Camera (`V`)** tiles (persisted in `~/.config/meetjoiner/preferences.json`, defaulting to both off)
  - **Snooze 2m (`S`)**, **Join Meeting (`Return`)**, and **Dismiss (`Esc` / `✕`)**
- When you click **Join Meeting**, opens the Meet link in Chrome, sets your chosen mic/camera state on the pre-join screen, and clicks "Join now"

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
