package radiko

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/yyoshiki41/go-radiko/internal/m3u8"
	"github.com/yyoshiki41/go-radiko/internal/util"
)

const timeshiftPlaylistEndpoint = "https://tf-f-rpaa-radiko.smartstream.ne.jp/tf/playlist.m3u8"

// TimeshiftPlaylistM3U8 returns uri.
func (c *Client) TimeshiftPlaylistM3U8(ctx context.Context, stationID string, start time.Time) (string, error) {
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

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return "", err
	}
	if ctx == nil {
		return "", errors.New("Context is nil")
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("pragma", "no-cache")
	if c.AuthToken() != "" {
		req.Header.Set(radikoAuthTokenHeader, c.AuthToken())
	}

	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return m3u8.GetURI(resp.Body)
}

// GetTimeshiftURL returns a timeshift url for web browser.
func GetTimeshiftURL(stationID string, start time.Time) string {
	endpoint := path.Join("#!/ts", stationID, util.Datetime(start))
	return defaultEndpoint + "/" + endpoint
}
