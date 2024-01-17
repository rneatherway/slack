# slack

[![CI](https://github.com/rneatherway/slack/actions/workflows/ci.yml/badge.svg)](https://github.com/rneatherway/slack-client/actions/workflows/ci.yml)

The code in this repository was split out from
https://github.com/rneatherway/gh-slack to make it easier for people to write
their own Go programs interacting with the Slack API without having to create a
Slack App.

The intended usage is:

```golang
client := slack.NewClient("your-team-here")
err := client.WithCookieAuth()
if err != nil {
    return nil, err
}

bs, err := client.API(context.TODO(), "GET", "users.list", nil, nil)
if err != nil {
    return nil, err
}

// Do something with `bs`...
```

The `WithCookieAuth()` method extracts a cookie from your system keychain and
uses it to fetch a token from `slack.com`. This is the main benefit of this
package. You can also obtain the token and cookie for your own use elsewhere
using the `GetCookieAuth(...)` function.

If you have an [access token](https://api.slack.com/authentication/token-types)
already and still want to use this simple `API(..)` method you can use this with
`client.GetTokenAuth(...)`.