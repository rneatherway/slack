package markdown

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var userRE = regexp.MustCompile("<@[A-Z0-9]+>")
var linkRE = regexp.MustCompile(`<(https?://[^|>]+)\|([^>]+)>`)
var openCodefence = regexp.MustCompile("(?m)^```")
var closeCodefence = regexp.MustCompile("(?m)(.)```$")

type UserProvider interface {
	UsernameForID(string) (string, error)
}

func interpolateUsers(client UserProvider, s string) (string, error) {
	userLocations := userRE.FindAllStringIndex(s, -1)
	out := &strings.Builder{}
	last := 0
	for _, userLocation := range userLocations {
		start := userLocation[0]
		end := userLocation[1]

		username, err := client.UsernameForID(s[start+2 : end-1])
		if err != nil {
			return "", err
		}
		out.WriteString(s[last:start])
		out.WriteString("`@")
		out.WriteString(username)
		out.WriteRune('`')
		last = end
	}
	out.WriteString(s[last:])

	return out.String(), nil
}

func ParseUnixTimestamp(s string) (*time.Time, error) {
	tsParts := strings.Split(s, ".")
	if len(tsParts) != 2 {
		return nil, fmt.Errorf("timestamp '%s' is not in <seconds>.<milliseconds> format", s)
	}

	seconds, err := strconv.ParseInt(tsParts[0], 10, 64)
	if err != nil {
		return nil, err
	}

	nanos, err := strconv.ParseInt(tsParts[1], 10, 64)
	if err != nil {
		return nil, err
	}

	result := time.Unix(seconds, nanos)
	return &result, nil
}

func Convert(client UserProvider, s string) (string, error) {
	text, err := interpolateUsers(client, s)
	if err != nil {
		return "", err
	}

	text = linkRE.ReplaceAllString(text, "[$2]($1)")
	text = openCodefence.ReplaceAllString(text, "```\n")
	text = closeCodefence.ReplaceAllString(text, "$1\n```")

	return text, nil
}

func PrefixEachLine(prefix, s string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}

func WrapInDetails(summary, s string) string {
	return fmt.Sprintf(
		"<details>\n<summary>%s</summary>\n\n%s\n</details>",
		summary, s)
}
