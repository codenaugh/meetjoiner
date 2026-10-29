package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	pollInterval    = 30 * time.Second
	lookAheadWindow = 5 * time.Minute
)

const (
	launchAgentLabel = "com.meetjoiner"
	launchAgentDir   = "Library/LaunchAgents"
	launchAgentFile  = launchAgentLabel + ".plist"
	appBundleRelPath = "Library/Application Support/MeetJoiner.app"
)

//go:embed Info.plist
var embeddedInfoPlist []byte

var calendarID = flag.String("calendar", "all", "calendar name or account to watch (defaults to all synced calendars)")

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "install":
			if err := cmdInstall(); err != nil {
				log.Fatalf("install failed: %v", err)
			}
			return
		case "uninstall":
			if err := cmdUninstall(); err != nil {
				log.Fatalf("uninstall failed: %v", err)
			}
			return
		case "calendars":
			if err := cmdCalendars(); err != nil {
				log.Fatalf("calendars failed: %v", err)
			}
			return
		case "test-popup":
			if err := cmdTestPopup(); err != nil {
				log.Fatalf("test-popup failed: %v", err)
			}
			return
		case "popup-ui":
			runtime.LockOSThread()
			cmdPopupUI(args[1:])
			return
		default:
			log.Fatalf("unknown command: %s (expected install, uninstall, calendars, or test-popup)", args[0])
		}
	}

	log.Printf("meetjoiner starting up (calendar=%s)", *calendarID)

	cfgDir, err := configDir()
	if err != nil {
		log.Fatalf("failed to determine config dir: %v", err)
	}
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		log.Fatalf("failed to create config dir: %v", err)
	}

	if err := ensureCalendarAccess(); err != nil {
		log.Fatalf("calendar access failed: %v", err)
	}

	log.Println("macOS Calendar access granted — polling calendar")
	poller := &meetPoller{
		configDir: cfgDir,
		scheduled: make(map[string]bool),
	}
	poller.run(context.Background())
}

func cmdCalendars() error {
	cals, err := getCalendars()
	if err != nil {
		return err
	}
	if len(cals) == 0 {
		fmt.Println("No calendars found in macOS Calendar.")
		return nil
	}
	fmt.Println("Synced macOS Calendars:")
	for _, c := range cals {
		fmt.Printf("  • %s (Account: %s)\n", c.Title, c.Source)
	}
	return nil
}

func cmdInstall() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("determine executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve symlinks: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("determine home dir: %w", err)
	}

	// Build a lightweight background .app bundle so macOS TCC can prompt and
	// grant Calendar permission to MeetJoiner.app independently of Terminal/IDE.
	appDir := filepath.Join(home, appBundleRelPath)
	macOSDir := filepath.Join(appDir, "Contents", "MacOS")
	infoPlistPath := filepath.Join(appDir, "Contents", "Info.plist")
	bundleExe := filepath.Join(macOSDir, "meetjoiner")

	if err := os.MkdirAll(macOSDir, 0o755); err != nil {
		return fmt.Errorf("create app bundle dir: %w", err)
	}
	if err := os.WriteFile(infoPlistPath, embeddedInfoPlist, 0o644); err != nil {
		return fmt.Errorf("write bundle Info.plist: %w", err)
	}
	if exe != bundleExe {
		if err := copyFile(exe, bundleExe, 0o755); err != nil {
			return fmt.Errorf("copy binary to app bundle: %w", err)
		}
	}
	if out, err := exec.Command("codesign", "-s", "-", "--force", "--deep", appDir).CombinedOutput(); err != nil {
		return fmt.Errorf("codesign app bundle: %w (%s)", err, out)
	}

	// Trigger the macOS Calendar permission check via LaunchServices so TCC
	// attributes the prompt to MeetJoiner.app.
	_ = exec.Command("open", "-W", appDir, "--args", "calendars").Run()

	plistPath := filepath.Join(home, launchAgentDir, launchAgentFile)

	progArgs := fmt.Sprintf(`        <string>%s</string>`, bundleExe)
	if *calendarID != "all" && *calendarID != "" {
		progArgs += fmt.Sprintf(`
        <string>-calendar</string>
        <string>%s</string>`, *calendarID)
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
%s
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/meetjoiner.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/meetjoiner.log</string>
</dict>
</plist>
`, launchAgentLabel, progArgs)

	_ = exec.Command("launchctl", "unload", plistPath).Run()

	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}

	fmt.Printf("Installed and started LaunchAgent\n")
	fmt.Printf("  app:      %s\n", appDir)
	fmt.Printf("  plist:    %s\n", plistPath)
	fmt.Printf("  calendar: %s\n", *calendarID)
	fmt.Printf("  logs:     /tmp/meetjoiner.log\n")
	return nil
}

func cmdUninstall() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("determine home dir: %w", err)
	}

	plistPath := filepath.Join(home, launchAgentDir, launchAgentFile)

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		fmt.Println("LaunchAgent not installed, nothing to do.")
	} else {
		if err := exec.Command("launchctl", "unload", plistPath).Run(); err != nil {
			fmt.Printf("warning: launchctl unload: %v\n", err)
		}
		if err := os.Remove(plistPath); err != nil {
			return fmt.Errorf("remove plist: %w", err)
		}
		fmt.Printf("Uninstalled LaunchAgent (removed %s)\n", plistPath)
	}

	appDir := filepath.Join(home, appBundleRelPath)
	if _, err := os.Stat(appDir); err == nil {
		_ = os.RemoveAll(appDir)
		fmt.Printf("Removed %s\n", appDir)
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "meetjoiner"), nil
}

type userPrefs struct {
	AudioOn bool `json:"audio_on"`
	VideoOn bool `json:"video_on"`
}

func prefsPath(cfgDir string) string {
	return filepath.Join(cfgDir, "preferences.json")
}

// loadPrefs loads the last saved audio/video settings, defaulting to both off.
func loadPrefs(cfgDir string) userPrefs {
	f, err := os.Open(prefsPath(cfgDir))
	if err != nil {
		return userPrefs{AudioOn: false, VideoOn: false}
	}
	defer f.Close()
	var p userPrefs
	if err := json.NewDecoder(f).Decode(&p); err != nil {
		return userPrefs{AudioOn: false, VideoOn: false}
	}
	return p
}

func savePrefs(cfgDir string, p userPrefs) error {
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(prefsPath(cfgDir), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(p)
}

func cmdTestPopup() error {
	cfgDir, err := configDir()
	if err != nil {
		return err
	}
	prefs := loadPrefs(cfgDir)
	res, err := promptJoinDialog(
		"Weekly Product & Engineering Sync",
		"with Sarah Jenkins, Alex Rivera + 3 others",
		time.Now(),
		prefs,
	)
	if err != nil {
		return err
	}
	if err := savePrefs(cfgDir, userPrefs{AudioOn: res.AudioOn, VideoOn: res.VideoOn}); err != nil {
		log.Printf("failed to save preferences: %v", err)
	}
	fmt.Printf("Result: action=%s, audio_on=%v, video_on=%v (saved to %s)\n",
		res.Action, res.AudioOn, res.VideoOn, prefsPath(cfgDir))
	return nil
}

type meetPoller struct {
	configDir string
	mu        sync.Mutex
	scheduled map[string]bool // event IDs we've already scheduled/prompted
}

func (p *meetPoller) run(ctx context.Context) {
	p.check(ctx)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.check(ctx)
		}
	}
}

func (p *meetPoller) check(ctx context.Context) {
	now := time.Now()
	tMin := now.Add(-1 * time.Minute)
	tMax := now.Add(lookAheadWindow)

	events, err := fetchUpcomingEvents(*calendarID, tMin, tMax)
	if err != nil {
		log.Printf("calendar fetch error: %v", err)
		return
	}

	for _, event := range events {
		p.maybeSchedule(ctx, event)
	}
}

func (p *meetPoller) maybeSchedule(ctx context.Context, event localEvent) {
	meetLink := extractMeetLinkFromEvent(event)
	if meetLink == "" {
		return
	}

	start := time.Unix(event.StartUnix, 0)
	diff := time.Until(start)
	if diff > lookAheadWindow || diff < -5*time.Minute {
		return
	}

	p.mu.Lock()
	if p.scheduled[event.ID] {
		p.mu.Unlock()
		return
	}
	if len(p.scheduled) > 200 {
		p.scheduled = make(map[string]bool)
	}
	p.scheduled[event.ID] = true
	p.mu.Unlock()

	subtitle := formatAttendeeList(event.Attendees)
	go p.promptAtStart(ctx, event.Summary, subtitle, meetLink, start)
}

func (p *meetPoller) promptAtStart(ctx context.Context, summary, subtitle, meetLink string, start time.Time) {
	if wait := time.Until(start); wait > 0 {
		log.Printf("scheduled join prompt for %q at %s (in %s)", summary, start.Format(time.Kitchen), wait.Round(time.Second))
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}

	for {
		if isMeetAlreadyOpen(meetLink) {
			log.Printf("already in meeting: %s — skipping", summary)
			return
		}

		prefs := loadPrefs(p.configDir)
		log.Printf("prompting to join meeting: %s (%s)", summary, meetLink)
		res, err := promptJoinDialog(summary, subtitle, start, prefs)
		if err != nil {
			log.Printf("failed to display join prompt for %q: %v", summary, err)
			return
		}

		if err := savePrefs(p.configDir, userPrefs{AudioOn: res.AudioOn, VideoOn: res.VideoOn}); err != nil {
			log.Printf("failed to save preferences: %v", err)
		}

		switch res.Action {
		case "snooze":
			log.Printf("snoozed meeting %q for 2 minutes", summary)
			snoozeTimer := time.NewTimer(2 * time.Minute)
			select {
			case <-ctx.Done():
				snoozeTimer.Stop()
				return
			case <-snoozeTimer.C:
				continue
			}
		case "join":
			log.Printf("joining meeting: %s (%s) [audio=%v, video=%v]", summary, meetLink, res.AudioOn, res.VideoOn)
			if err := openMeet(meetLink, res.AudioOn, res.VideoOn); err != nil {
				log.Printf("failed to open %s: %v", meetLink, err)
			}
			return
		default:
			log.Printf("dismissed meeting: %s", summary)
			return
		}
	}
}

type dialogResult struct {
	Action  string `json:"action"`
	AudioOn bool   `json:"audio_on"`
	VideoOn bool   `json:"video_on"`
}

func cmdPopupUI(args []string) {
	summary := "Google Meet"
	subtitle := ""
	startStr := time.Now().Local().Format("3:04 PM")
	startUnix := time.Now().Unix()
	var defaults userPrefs
	if len(args) > 0 {
		summary = args[0]
	}
	if len(args) > 1 {
		subtitle = args[1]
	}
	if len(args) > 2 {
		startStr = args[2]
	}
	if len(args) > 3 {
		if v, err := strconv.ParseInt(args[3], 10, 64); err == nil {
			startUnix = v
		}
	}
	if len(args) > 4 {
		defaults.AudioOn = args[4] == "true"
	}
	if len(args) > 5 {
		defaults.VideoOn = args[5] == "true"
	}
	res := runPopupUI(summary, subtitle, startStr, startUnix, defaults)
	_ = json.NewEncoder(os.Stdout).Encode(res)
}

func promptJoinDialog(summary, subtitle string, start time.Time, defaults userPrefs) (dialogResult, error) {
	if summary == "" {
		summary = "Google Meet"
	}
	exe, err := os.Executable()
	if err != nil {
		return dialogResult{}, fmt.Errorf("determine executable path: %w", err)
	}

	out, err := exec.Command(
		exe, "popup-ui",
		summary,
		subtitle,
		start.Local().Format("3:04 PM"),
		strconv.FormatInt(start.Unix(), 10),
		strconv.FormatBool(defaults.AudioOn),
		strconv.FormatBool(defaults.VideoOn),
	).CombinedOutput()
	if err != nil {
		return dialogResult{}, fmt.Errorf("%w: %s", err, out)
	}

	var res dialogResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &res); err != nil {
		return dialogResult{}, fmt.Errorf("parse dialog output %q: %w", string(out), err)
	}
	return res, nil
}

// isMeetAlreadyOpen checks if the user is already in this meeting in Chrome.
func isMeetAlreadyOpen(meetLink string) bool {
	script := `
tell application "Google Chrome"
	repeat with w in windows
		set tabCount to count of tabs of w
		repeat with i from 1 to tabCount
			set t to tab i of w
			if URL of t contains "` + meetLink + `" then
				return "found"
			end if
		end repeat
	end repeat
	return "not found"
end tell`

	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return false
	}
	return string(out) == "found\n"
}

// openMeet opens a Google Meet link in Chrome, configures mic/camera according
// to audioOn and videoOn, and clicks "Join now."
func openMeet(meetLink string, audioOn, videoOn bool) error {
	micLabel := "Turn off microphone"
	if audioOn {
		micLabel = "Turn on microphone"
	}
	camLabel := "Turn off camera"
	if videoOn {
		camLabel = "Turn on camera"
	}

	fallbackMute := ""
	if !audioOn && !videoOn {
		fallbackMute = `
	-- Fallback: click any unmuted data-is-muted toggles.
	try
		execute meetTab javascript "document.querySelectorAll('[data-is-muted=\"false\"]').forEach(function(b) { b.click(); });"
	end try

	delay 0.5
`
	}

	script := `
tell application "Google Chrome"
	activate
	if (count of windows) is 0 then
		make new window
	end if
	-- Open a new tab, then navigate to the Meet URL.
	-- (Setting URL after creation avoids Meet redirecting to the landing page.)
	tell front window
		set newTab to make new tab
		set URL of newTab to "` + meetLink + `"
	end tell
end tell

delay 7

tell application "Google Chrome"
	activate

	-- Find the Meet tab by URL.
	set meetTab to missing value
	repeat with w in windows
		set tabCount to count of tabs of w
		repeat with i from 1 to tabCount
			set t to tab i of w
			if URL of t contains "meet.google.com/" and URL of t is not "https://meet.google.com/landing" then
				set meetTab to t
				set active tab index of w to i
				exit repeat
			end if
		end repeat
	end repeat

	if meetTab is missing value then return "no meet tab"

	-- If the page redirected to the landing page, navigate again.
	if URL of meetTab is "https://meet.google.com/landing" then
		set URL of meetTab to "` + meetLink + `"
		delay 7
	end if

	-- Wait up to 6 seconds for the pre-join controls to render.
	repeat 6 times
		try
			set ready to execute meetTab javascript "
				!!(document.querySelector('[aria-label*=\"Turn off microphone\"]') ||
				   document.querySelector('[aria-label*=\"Turn on microphone\"]') ||
				   document.querySelector('[data-is-muted]'))
			"
			if ready is "true" then exit repeat
		end try
		delay 1
	end repeat

	-- Set microphone state.
	try
		execute meetTab javascript "var m = document.querySelector('[aria-label*=\"` + micLabel + `\"]'); if (m) m.click();"
	end try

	delay 0.5

	-- Set camera state.
	try
		execute meetTab javascript "var c = document.querySelector('[aria-label*=\"` + camLabel + `\"]'); if (c) c.click();"
	end try

	delay 0.5
` + fallbackMute + `
	-- Click "Join now" (or "Ask to join").
	try
		execute meetTab javascript "
			(function() {
				var buttons = document.querySelectorAll('button');
				for (var i = 0; i < buttons.length; i++) {
					var txt = buttons[i].textContent.trim();
					if (txt === 'Join now' || txt === 'Ask to join') {
						buttons[i].click();
						return;
					}
				}
			})();
		"
	end try
end tell`

	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}
