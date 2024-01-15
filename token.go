package slack

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/rneatherway/slackclient/internal/config"
	_ "modernc.org/sqlite"
)

var apiTokenRE = regexp.MustCompile("\"api_token\":\"([^\"]+)\"")

type Auth struct {
	Token   string
	Cookies map[string]string
}

func GetAuth(team string) (*Auth, error) {
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

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	matches := apiTokenRE.FindSubmatch(bs)
	if matches == nil {
		return nil, errors.New("api token not found")
	}

	return &Auth{Token: string(matches[1]), Cookies: map[string]string{"d": cookie}}, nil
}
