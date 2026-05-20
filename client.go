package slack

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	team string
	auth Auth

	httpClient *http.Client
}

func NewClient(team string) *Client {
	return &Client{
		team:       team,
		httpClient: http.DefaultClient,
	}
}

func (c *Client) WithCookieAuth() error {
	if auth, ok := TryGetEnvAuth(); ok {
		c.auth = *auth
		return nil
	}

	auth, err := GetCookieAuth(c.team)
	if err != nil {
		return err
	}

	c.auth = *auth
	return nil
}

func (c *Client) WithTokenAuth(token string) {
	c.auth = Auth{Token: token}
}

func (c *Client) WithHTTPClient(httpClient *http.Client) {
	c.httpClient = httpClient
}

func (c *Client) API(ctx context.Context, verb, path string, params map[string]string, body []byte) ([]byte, error) {
	u, err := c.buildURL(path, params)
	if err != nil {
		return nil, err
	}

	makeReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, verb, u.String(), bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		// FIXME: this doesn't seem to break non-POST/non-data requests, but might
		// be polluting the headers.
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		return req, nil
	}

	return c.doWithRetry(makeReq)
}

// FileParam represents a file to upload in a multipart request.
type FileParam struct {
	// Fieldname is the form field name (e.g. "image").
	Fieldname string
	// Filename is the name of the file as it will appear to the server.
	Filename string
	// Reader provides the file content.
	Reader io.Reader
}

// APIMultipart sends a multipart/form-data request to the Slack API.
// This is required for endpoints that accept file uploads, such as
// users.setPhoto and files.upload.
func (c *Client) APIMultipart(ctx context.Context, path string, params map[string]string, file FileParam) ([]byte, error) {
	u, err := c.buildURL(path, nil)
	if err != nil {
		return nil, err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for key, val := range params {
		if err := writer.WriteField(key, val); err != nil {
			return nil, fmt.Errorf("writing field %q: %w", key, err)
		}
	}

	part, err := writer.CreateFormFile(file.Fieldname, file.Filename)
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(part, file.Reader); err != nil {
		return nil, fmt.Errorf("copying file data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	contentType := writer.FormDataContentType()
	bodyBytes := body.Bytes()

	makeReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", contentType)
		return req, nil
	}

	return c.doWithRetry(makeReq)
}

func (c *Client) buildURL(path string, params map[string]string) (*url.URL, error) {
	u, err := url.Parse(fmt.Sprintf("https://%s.slack.com/api/", c.team))
	if err != nil {
		return nil, err
	}
	u.Path += path
	q := u.Query()
	for p := range params {
		q.Add(p, params[p])
	}
	u.RawQuery = q.Encode()
	return u, nil
}

func (c *Client) doWithRetry(makeReq func() (*http.Request, error)) ([]byte, error) {
	for {
		req, err := makeReq()
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.auth.Token))
		for key := range c.auth.Cookies {
			req.AddCookie(&http.Cookie{Name: key, Value: c.auth.Cookies[key]})
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		resBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == 429 {
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter == "" {
				return nil, fmt.Errorf("status code 429 without Retry-After header")
			}
			s, err := strconv.Atoi(retryAfter)
			if err != nil {
				return nil, err
			}
			d := time.Duration(s)
			time.Sleep(d * time.Second)
		} else if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("status code %d, headers: %q, body: %q", resp.StatusCode, resp.Header, resBody)
		} else {
			return resBody, nil
		}
	}
}
