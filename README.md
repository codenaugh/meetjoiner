# meetjoiner

A background daemon that automatically joins your Google Meet meetings with mic and camera off.

It watches your Google Calendar for accepted meetings and, 2 minutes after the scheduled start time, opens the Meet link in Chrome, mutes mic/camera, and clicks "Join now."

## Requirements

- **macOS** (uses AppleScript to control Chrome)
- **Google Chrome** with JavaScript from Apple Events enabled:
  - Chrome menu bar → **View** → **Developer** → **Allow JavaScript from Apple Events**
- **Google Calendar API OAuth 2.0 credentials** (Desktop app type) from a [Google Cloud project](https://console.cloud.google.com/apis/credentials) with the Google Calendar API enabled

## Setup

1. **Place your OAuth credentials:**

   ```sh
   mkdir -p ~/.config/meetjoiner
   cp /path/to/client_secret_XXXXX.json ~/.config/meetjoiner/credentials.json
   ```

   The JSON should be an OAuth 2.0 Client ID of type "Desktop app." If you already have one (e.g., from another tool), you can reuse it.

2. **Build:**

   ```sh
   go build -o meetjoiner .
   ```

3. **First run (authenticates via browser):**

   ```sh
   ./meetjoiner -calendar you@example.com
   ```

   A browser window will open for Google OAuth consent. After you approve, the token is saved to `~/.config/meetjoiner/token.json` and the daemon begins polling.

4. **Install as a LaunchAgent (runs on login):**

   First, edit `com.meetjoiner.plist` and replace the placeholder values:

   - Change `/path/to/meetjoiner` to the absolute path to your built binary (e.g., `/Users/you/meetjoiner/meetjoiner`)
   - Add `-calendar` and your calendar ID (typically your email) as additional `<string>` entries in the `ProgramArguments` array

   For example:
   ```xml
   <key>ProgramArguments</key>
   <array>
       <string>/Users/you/meetjoiner/meetjoiner</string>
       <string>-calendar</string>
       <string>you@example.com</string>
   </array>
   ```

   Then install it:

   ```sh
   cp com.meetjoiner.plist ~/Library/LaunchAgents/
   launchctl load ~/Library/LaunchAgents/com.meetjoiner.plist
   ```

## Usage

```
./meetjoiner -calendar <calendar-id>
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

# Remove
launchctl unload ~/Library/LaunchAgents/com.meetjoiner.plist
rm ~/Library/LaunchAgents/com.meetjoiner.plist
```
