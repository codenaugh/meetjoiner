package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	pollInterval = 30 * time.Second
	// How far ahead to look for events.
	lookAheadWindow = 5 * time.Minute
)

const (
	launchAgentLabel = "com.meetjoiner"
	launchAgentDir   = "Library/LaunchAgents"
	launchAgentFile  = launchAgentLabel + ".plist"
)

var calendarID = flag.String("calendar", "primary", "calendar ID to watch (e.g. user@example.com)")

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
		case "auth":
			if err := cmdAuth(); err != nil {
				log.Fatalf("auth failed: %v", err)
			}
			return
		default:
			log.Fatalf("unknown command: %s (expected install, uninstall, or auth)", args[0])
		}
	}

	log.Printf("meetjoiner starting up (calendar=%s)", *calendarID)

	configDir, err := configDir()
	if err != nil {
		log.Fatalf("failed to determine config dir: %v", err)
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		log.Fatalf("failed to create config dir: %v", err)
	}

	credFile := filepath.Join(configDir, "credentials.json")
	if _, err := os.Stat(credFile); os.IsNotExist(err) {
		log.Fatalf("missing %s — download OAuth client credentials from Google Cloud Console and place them there", credFile)
	}

	ctx := context.Background()
	client, err := getClient(ctx, configDir, credFile)
	if err != nil {
		log.Fatalf("failed to get OAuth client: %v", err)
	}

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("failed to create calendar service: %v", err)
	}

	log.Println("authenticated — polling calendar")
	poller := &meetPoller{
		srv:    srv,
		opened: make(map[string]bool),
	}
	poller.run(ctx)
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

	plistPath := filepath.Join(home, launchAgentDir, launchAgentFile)

	// Build ProgramArguments.
	progArgs := fmt.Sprintf(`        <string>%s</string>`, exe)
	if *calendarID != "primary" {
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

	// Unload existing agent if present (ignore errors).
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
	fmt.Printf("  plist: %s\n", plistPath)
	fmt.Printf("  binary: %s\n", exe)
	fmt.Printf("  calendar: %s\n", *calendarID)
	fmt.Printf("  logs: /tmp/meetjoiner.log\n")
	return nil
}

func cmdAuth() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	credFile := filepath.Join(dir, "credentials.json")
	if _, err := os.Stat(credFile); os.IsNotExist(err) {
		return fmt.Errorf("missing %s — download OAuth client credentials from Google Cloud Console and place them there", credFile)
	}

	tokFile := filepath.Join(dir, "token.json")
	if _, err := loadToken(tokFile); err == nil {
		fmt.Println("Already authenticated (token exists at " + tokFile + ").")
		fmt.Println("Delete it and re-run auth to re-authenticate.")
		return nil
	}

	b, err := os.ReadFile(credFile)
	if err != nil {
		return fmt.Errorf("read credentials: %w", err)
	}
	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		return fmt.Errorf("parse credentials: %w", err)
	}

	ctx := context.Background()
	tok, err := getTokenFromWeb(ctx, config)
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}
	if err := saveToken(tokFile, tok); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	fmt.Println("Authenticated successfully! Token saved to " + tokFile)
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
		return nil
	}

	if err := exec.Command("launchctl", "unload", plistPath).Run(); err != nil {
		fmt.Printf("warning: launchctl unload: %v\n", err)
	}
	if err := os.Remove(plistPath); err != nil {
		return fmt.Errorf("remove plist: %w", err)
	}

	fmt.Printf("Uninstalled LaunchAgent (removed %s)\n", plistPath)
	return nil
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "meetjoiner"), nil
}

// getClient returns an HTTP client with a valid OAuth2 token.
// On first run it opens the browser for consent.
func getClient(ctx context.Context, configDir, credFile string) (*http.Client, error) {
	b, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	tokFile := filepath.Join(configDir, "token.json")
	tok, err := loadToken(tokFile)
	if err != nil {
		tok, err = getTokenFromWeb(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("get token from web: %w", err)
		}
		if err := saveToken(tokFile, tok); err != nil {
			return nil, fmt.Errorf("save token: %w", err)
		}
	}

	return config.Client(ctx, tok), nil
}

func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	return tok, json.NewDecoder(f).Decode(tok)
}

func saveToken(path string, tok *oauth2.Token) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(tok)
}

func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	// Use a local redirect so the user doesn't have to copy-paste a code.
	config.RedirectURL = "http://localhost:8089/callback"

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Opening browser for Google authorization...\n")
	_ = exec.Command("open", authURL).Start()
	fmt.Printf("If the browser didn't open, visit:\n%s\n", authURL)

	codeCh := make(chan string, 1)
	srv := &http.Server{Addr: ":8089"}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		codeCh <- code
		fmt.Fprintf(w, "<h1>Authorized! You can close this tab.</h1>")
	})

	go func() { _ = srv.ListenAndServe() }()
	code := <-codeCh
	_ = srv.Shutdown(ctx)

	return config.Exchange(ctx, code)
}

type meetPoller struct {
	srv    *calendar.Service
	mu     sync.Mutex
	opened map[string]bool // event IDs we've already opened
}

func (p *meetPoller) run(ctx context.Context) {
	// Do an immediate check, then tick.
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
	tMin := now.Format(time.RFC3339)
	tMax := now.Add(lookAheadWindow).Format(time.RFC3339)

	events, err := p.srv.Events.List(*calendarID).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(tMin).
		TimeMax(tMax).
		OrderBy("startTime").
		Do()
	if err != nil {
		log.Printf("calendar list error: %v", err)
		return
	}

	for _, event := range events.Items {
		p.maybeOpen(event, now)
	}
}

func (p *meetPoller) maybeOpen(event *calendar.Event, now time.Time) {
	// Only care about events with Google Meet links.
	meetLink := extractMeetLink(event)
	if meetLink == "" {
		return
	}

	// Only open events the user has accepted (or is the organizer).
	if !isAccepted(event) {
		return
	}

	// Parse the start time.
	start, err := parseEventTime(event.Start)
	if err != nil {
		log.Printf("failed to parse start time for %q: %v", event.Summary, err)
		return
	}

	// Join 2 minutes after start time (buffer for back-to-back meetings).
	joinAt := start.Add(2 * time.Minute)
	diff := time.Until(joinAt)
	if diff > 15*time.Second || diff < -5*time.Minute {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.opened[event.Id] {
		return
	}
	p.opened[event.Id] = true

	// Check if already in this meeting (a Chrome tab with this Meet URL is active and past the pre-join screen).
	if isMeetAlreadyOpen(meetLink) {
		log.Printf("already in meeting: %s — skipping", event.Summary)
		return
	}

	log.Printf("opening meeting: %s (%s)", event.Summary, meetLink)
	if err := openMeetMuted(meetLink); err != nil {
		log.Printf("failed to open %s: %v", meetLink, err)
	}

	// Prune old entries to avoid unbounded growth.
	if len(p.opened) > 200 {
		p.opened = make(map[string]bool)
	}
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

// openMeetMuted opens a Google Meet link in Chrome, mutes mic/camera, and
// clicks "Join now."
func openMeetMuted(meetLink string) error {
	script := `
tell application "Google Chrome"
	activate
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

	-- Mute mic if unmuted.
	try
		execute meetTab javascript "var m = document.querySelector('[aria-label*=\"Turn off microphone\"]'); if (m) m.click();"
	end try

	delay 0.5

	-- Mute camera if unmuted.
	try
		execute meetTab javascript "var c = document.querySelector('[aria-label*=\"Turn off camera\"]'); if (c) c.click();"
	end try

	delay 0.5

	-- Fallback: click any unmuted data-is-muted toggles.
	try
		execute meetTab javascript "document.querySelectorAll('[data-is-muted=\"false\"]').forEach(function(b) { b.click(); });"
	end try

	delay 0.5

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

func extractMeetLink(event *calendar.Event) string {
	if event.ConferenceData != nil {
		for _, ep := range event.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" && ep.Uri != "" {
				return ep.Uri
			}
		}
	}
	// Fallback: HangoutLink field.
	return event.HangoutLink
}

func isAccepted(event *calendar.Event) bool {
	if event.Attendees == nil {
		// No attendees list means the user is the sole organizer — treat as accepted.
		return true
	}
	for _, a := range event.Attendees {
		if a.Self && (a.ResponseStatus == "accepted" || a.ResponseStatus == "tentative") {
			return true
		}
	}
	return false
}

func parseEventTime(et *calendar.EventDateTime) (time.Time, error) {
	if et.DateTime != "" {
		return time.Parse(time.RFC3339, et.DateTime)
	}
	// All-day event — skip.
	return time.Time{}, fmt.Errorf("all-day event")
}
