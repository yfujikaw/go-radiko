package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"

	radiko "github.com/yyoshiki41/go-radiko"
)

func main() {
	stationID := flag.String("id", "LFR", "station id (e.g. LFR)")
	playerCmd := flag.String("player", "ffplay", "player command")
	dryRun := flag.Bool("dry-run", false, "print playlist URL only")
	flag.Parse()

	items, err := radiko.GetStreamSmhMultiURL(*stationID)
	if err != nil {
		log.Fatalf("failed to get stream info: %v", err)
	}

	playlistCreateURL, err := selectLivePlaylistCreateURL(items)
	if err != nil {
		log.Fatal(err)
	}
	client, err := radiko.New("")
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	authToken, err := client.AuthorizeToken(context.Background())
	if err != nil {
		log.Fatalf("failed to authorize token: %v", err)
	}
	playlistURL, err := resolveLivePlaylistURL(client, playlistCreateURL, authToken)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("playlist: %s\n", playlistURL)

	if *dryRun {
		return
	}

	cmd := exec.Command(*playerCmd, playlistURL)
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	log.Printf("start player: %s %s", *playerCmd, playlistURL)
	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to run player command: %v", err)
	}
}

func selectLivePlaylistCreateURL(items []radiko.SmhURLItem) (string, error) {
	// Prefer playlist_create endpoint because it returns a playable m3u URL.
	for _, item := range items {
		if strings.Contains(item.PlaylistCreateURL, "/v2/api/playlist_create/") && !item.Areafree {
			return item.PlaylistCreateURL, nil
		}
	}
	for _, item := range items {
		if strings.Contains(item.PlaylistCreateURL, "/v2/api/playlist_create/") {
			return item.PlaylistCreateURL, nil
		}
	}
	return "", fmt.Errorf("no playlist_create URL found in stream info")
}

func resolveLivePlaylistURL(client *radiko.Client, playlistCreateURL, authToken string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, playlistCreateURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Radiko-App", "pc_html5")
	req.Header.Set("X-Radiko-App-Version", "0.0.1")
	req.Header.Set("X-Radiko-User", "test-stream")
	req.Header.Set("X-Radiko-Device", "pc")
	req.Header.Set("X-Radiko-AuthToken", authToken)
	req.Header.Set("pragma", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("playlist_create failed: status=%d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	playlistURL := strings.TrimSpace(string(b))
	if playlistURL == "" {
		return "", fmt.Errorf("empty playlist URL from playlist_create")
	}
	return playlistURL, nil
}
