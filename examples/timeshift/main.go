package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	radiko "github.com/yyoshiki41/go-radiko"
)

const timeshiftLayout = "20060102150405"

func main() {
	stationID := flag.String("id", "LFR", "station id (e.g. LFR)")
	startAt := flag.String("s", "", "program start time in JST (YYYYMMDDhhmmss)")
	playerCmd := flag.String("player", "ffplay", "player command")
	dryRun := flag.Bool("dry-run", false, "print playlist URL only")
	flag.Parse()

	start, err := parseStartTime(*startAt)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	client, err := radiko.New("")
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	if _, err := client.AuthorizeToken(ctx); err != nil {
		log.Fatalf("failed to authorize token: %v", err)
	}

	// TimeshiftPlaylistM3U8 expects a program start time (ft), not an arbitrary timestamp.
	programStart, err := resolveProgramStartTime(ctx, client, *stationID, start)
	if err != nil {
		log.Fatalf("failed to resolve program start time: %v", err)
	}

	playlistURL, err := client.TimeshiftPlaylistM3U8(ctx, *stationID, programStart)
	if err != nil {
		log.Fatalf("failed to get timeshift playlist: %v", err)
	}
	fmt.Printf("playlist: %s\n", playlistURL)

	if *dryRun {
		return
	}

	playerArgs, cleanup, err := buildPlayerArgs(*playerCmd, playlistURL)
	if err != nil {
		log.Fatalf("failed to prepare player input: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	cmd := exec.Command(*playerCmd, playerArgs...)
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	log.Printf("start player: %s %s", *playerCmd, strings.Join(playerArgs, " "))
	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to run player command: %v", err)
	}
}

func parseStartTime(value string) (time.Time, error) {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return time.Time{}, err
	}
	if value == "" {
		return time.Time{}, fmt.Errorf("missing -s, use YYYYMMDDhhmmss (JST)")
	}
	t, err := time.ParseInLocation(timeshiftLayout, value, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid -s format, use YYYYMMDDhhmmss (JST): %w", err)
	}
	return t, nil
}

func resolveProgramStartTime(ctx context.Context, client *radiko.Client, stationID string, at time.Time) (time.Time, error) {
	stations, err := client.GetStations(ctx, at)
	if err != nil {
		return time.Time{}, err
	}
	for _, station := range stations {
		if station.ID != stationID {
			continue
		}
		for _, prog := range station.Progs.Progs {
			ft, err := parseRadikoDatetime(prog.Ft)
			if err != nil {
				continue
			}
			to, err := parseRadikoDatetime(prog.To)
			if err != nil {
				continue
			}
			if (at.Equal(ft) || at.After(ft)) && at.Before(to) {
				return ft, nil
			}
		}
		return time.Time{}, fmt.Errorf("no program found at %s for station %s", at.Format(timeshiftLayout), stationID)
	}
	return time.Time{}, fmt.Errorf("station not found: %s", stationID)
}

func parseRadikoDatetime(v string) (time.Time, error) {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return time.Time{}, err
	}
	return time.ParseInLocation(timeshiftLayout, v, loc)
}

func buildPlayerArgs(playerCmd, playlistURL string) ([]string, func(), error) {
	// ffplay cannot always auto-detect HLS from the "/tf/medialist" URL.
	if playerCmd == "ffplay" && strings.Contains(playlistURL, "/tf/medialist") {
		localM3U8, cleanup, err := downloadPlaylist(playlistURL)
		if err != nil {
			return nil, nil, err
		}
		return []string{"-protocol_whitelist", "file,http,https,tcp,tls", localM3U8}, cleanup, nil
	}
	return []string{playlistURL}, nil, nil
}

func downloadPlaylist(playlistURL string) (string, func(), error) {
	resp, err := http.Get(playlistURL)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	if !strings.Contains(string(b), "#EXTM3U") {
		return "", nil, fmt.Errorf("invalid playlist response")
	}

	f, err := os.CreateTemp("", "radiko-timeshift-*.m3u8")
	if err != nil {
		return "", nil, err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}
