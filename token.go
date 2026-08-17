package slack

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"github.com/rneatherway/slack/internal/config"
	_ "modernc.org/sqlite"
)

var apiTokenRE = regexp.MustCompile("\"api_token\":\"([^\"]+)\"")

const (
	EnvSlackToken   = "SLACK_TOKEN"
	EnvSlackCookies = "SLACK_COOKIES"
)

type Auth struct {
	Token   string
	Cookies map[string]string
}

func GetCookieAuth(team string) (*Auth, error) {
	cookie, err := config.GetCookie()
	if err != nil {
		return nil, err
	}

	r, err := http.NewRequest("GET", fmt.Sprintf("https://%s.slack.com", team), nil)
	if err != nil {
		return nil, err
	}

	r.AddCookie(&http.Cookie{Name: "d", Value: cookie})

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}

	// Note: here the resp.StatusCode is not checked for 200.
	// Slack would sometimes send back a 403 with a valid token.

	bs, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	matches := apiTokenRE.FindSubmatch(bs)
	if matches == nil {
		return nil, fmt.Errorf("api token not found, status code: %d", resp.StatusCode)
	}

	return &Auth{Token: string(matches[1]), Cookies: map[string]string{"d": cookie}}, nil
}

func TryGetEnvAuth() (*Auth, bool) {
	if slackToken, ok := os.LookupEnv(EnvSlackToken); ok {
		if slackCookies, ok := os.LookupEnv(EnvSlackCookies); ok {
			vals, err := url.ParseQuery(slackCookies)
			if err != nil {
				return nil, false
			}

			cookies := make(map[string]string, len(vals))
			for key, val := range vals {
				if len(val) != 1 {
					fmt.Fprintf(os.Stderr, "cookie %q has %d values: %q\n", key, len(val), val)
					return nil, false
				}

				cookies[key] = val[0]
			}

			return &Auth{
				Token:   slackToken,
				Cookies: cookies,
			}, true
		}
	}

	return nil, false
}
