package radiko

import (
	"bytes"
	"context"
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

	u, err := url.Parse(timeshiftPlaylistEndpoint)
	if err != nil {
		return "", err
	}
	query := u.Query()
	query.Set("station_id", stationID)
	query.Set("ft", prog.Ft)
	query.Set("to", prog.To)
	query.Set("l", "15") // must?
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

func (c *Client) requestTimeshiftPlaylistURI(ctx context.Context, method, endpoint string) (string, error) {
	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return "", err
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("pragma", "no-cache")
	req.Header.Set(radikoAppHeader, radikoApp)
	req.Header.Set(radikoAppVersionHeader, radikoAppVersion)
	req.Header.Set(radikoUserHeader, radikoUser)
	req.Header.Set(radikoDeviceHeader, radikoDevice)
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

// GetTimeshiftURL returns a timeshift url for web browser.
func GetTimeshiftURL(stationID string, start time.Time) string {
	endpoint := path.Join("#!/ts", stationID, util.Datetime(start))
	return defaultEndpoint + "/" + endpoint
}
