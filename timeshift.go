package radiko

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/yyoshiki41/go-radiko/internal/m3u8"
	"github.com/yyoshiki41/go-radiko/internal/util"
)

const timeshiftPlaylistEndpoint = "https://tf-f-rpaa-radiko.smartstream.ne.jp/tf/playlist.m3u8"

// TimeshiftPlaylistM3U8 returns uri.
func (c *Client) TimeshiftPlaylistM3U8(ctx context.Context, stationID string, start time.Time) (string, error) {
	if ctx == nil {
		return "", errors.New("Context is nil")
	}

	prog, err := c.GetProgramByStartTime(ctx, stationID, start)
	if err != nil {
		return "", err
	}

	endpoint, err := c.timeshiftPlaylistEndpoint(ctx, stationID)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	query := u.Query()
	query.Set("station_id", stationID)
	query.Set("start_at", prog.Ft)
	query.Set("ft", prog.Ft)
	query.Set("end_at", prog.To)
	query.Set("to", prog.To)
	query.Set("preroll", "2")
	query.Set("l", "15") // must?
	query.Set("lsid", randomLSID())
	query.Set("type", "b")
	u.RawQuery = query.Encode()

	methods := []string{"POST", "GET"}
	var lastErr error
	for _, method := range methods {
		uri, reqErr := c.requestTimeshiftPlaylistURI(ctx, method, u.String())
		if reqErr == nil {
			return uri, nil
		}
		lastErr = reqErr
	}
	return "", lastErr
}

func (c *Client) timeshiftPlaylistEndpoint(ctx context.Context, stationID string) (string, error) {
	apiEndpoint := path.Join(apiV3, "station/stream/pc_html5", stationID+".xml")
	req, err := c.newRequest(ctx, "GET", apiEndpoint, &Params{})
	if err != nil {
		return "", err
	}

	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to get station stream info: status=%d", resp.StatusCode)
	}

	var data stationStreamData
	if err := xml.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	fallback := ""
	for _, u := range data.URLs {
		if u.Timefree != "1" || u.PlaylistCreateURL == "" {
			continue
		}
		// Prefer area-locked endpoint first for non-premium flow compatibility.
		if u.Arefree == "0" {
			return u.PlaylistCreateURL, nil
		}
		if fallback == "" {
			fallback = u.PlaylistCreateURL
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return timeshiftPlaylistEndpoint, nil
}

type stationStreamData struct {
	URLs []stationStreamURL `xml:"url"`
}

type stationStreamURL struct {
	Arefree           string `xml:"areafree,attr"`
	Timefree          string `xml:"timefree,attr"`
	PlaylistCreateURL string `xml:"playlist_create_url"`
}

func (c *Client) requestTimeshiftPlaylistURI(ctx context.Context, method, endpoint string) (string, error) {
	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return "", err
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("Origin", defaultEndpoint)
	req.Header.Set("Referer", defaultEndpoint+"/")
	req.Header.Set(radikoAppHeader, radikoApp)
	req.Header.Set(radikoAppVersionHeader, radikoAppVersion)
	req.Header.Set(radikoUserHeader, radikoUser)
	req.Header.Set(radikoDeviceHeader, radikoDevice)
	if c.AreaID() != "" {
		req.Header.Set("X-Radiko-AreaId", c.AreaID())
	}
	if c.AuthToken() != "" {
		req.Header.Set(radikoAuthTokenHeader, c.AuthToken())
	}

	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("failed to get playlist.m3u8 with %s: status=%d body=%q", method, resp.StatusCode, snippet(body))
	}

	uri, err := m3u8.GetURI(bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("invalid playlist response with %s: %w (body=%q)", method, err, snippet(body))
	}
	return uri, nil
}

func snippet(body []byte) string {
	s := strings.TrimSpace(string(body))
	const max = 200
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func randomLSID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// fallback keeps behavior deterministic if entropy is unavailable.
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}

// GetTimeshiftURL returns a timeshift url for web browser.
func GetTimeshiftURL(stationID string, start time.Time) string {
	endpoint := path.Join("#!/ts", stationID, util.Datetime(start))
	return defaultEndpoint + "/" + endpoint
}
