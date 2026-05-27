package slack

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c := &Client{
		team:       "test",
		auth:       Auth{Token: "xoxc-test-token"},
		httpClient: srv.Client(),
	}
	// Override buildURL to point at the test server instead of slack.com.
	// We can't do that directly, so we'll use a custom transport.
	c.httpClient = &http.Client{
		Transport: &rewriteTransport{
			base:    srv.Client().Transport,
			baseURL: srv.URL,
		},
	}
	return c
}

// rewriteTransport redirects all requests to the test server.
type rewriteTransport struct {
	base    http.RoundTripper
	baseURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.baseURL, "http://")
	if t.base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return t.base.RoundTrip(req)
}

func TestAPI_SendsJSONWithAuth(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer xoxc-test-token" {
			t.Errorf("Authorization = %q, want Bearer xoxc-test-token", got)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"hello":"world"}` {
			t.Errorf("body = %q, want {\"hello\":\"world\"}", body)
		}
		w.Write([]byte(`{"ok":true}`))
	})

	c := testClient(t, handler)
	resp, err := c.API(context.Background(), "POST", "chat.postMessage", nil, []byte(`{"hello":"world"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(resp) != `{"ok":true}` {
		t.Errorf("response = %q, want {\"ok\":true}", resp)
	}
}

func TestAPI_PassesQueryParams(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("channel"); got != "C123" {
			t.Errorf("channel param = %q, want C123", got)
		}
		w.Write([]byte(`{"ok":true}`))
	})

	c := testClient(t, handler)
	_, err := c.API(context.Background(), "GET", "conversations.info",
		map[string]string{"channel": "C123"}, []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestAPIMultipart_SendsFileAndParams(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer xoxc-test-token" {
			t.Errorf("Authorization = %q, want Bearer xoxc-test-token", got)
		}

		ct := r.Header.Get("Content-Type")
		mediaType, mpParams, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatalf("parsing Content-Type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("media type = %q, want multipart/form-data", mediaType)
		}

		reader := multipart.NewReader(r.Body, mpParams["boundary"])

		var sawFile bool
		fields := map[string]string{}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if part.FileName() != "" {
				sawFile = true
				if part.FormName() != "image" {
					t.Errorf("file field name = %q, want image", part.FormName())
				}
				if part.FileName() != "photo.jpg" {
					t.Errorf("file name = %q, want photo.jpg", part.FileName())
				}
				data, _ := io.ReadAll(part)
				if string(data) != "fake-image-data" {
					t.Errorf("file data = %q, want fake-image-data", data)
				}
			} else {
				val, _ := io.ReadAll(part)
				fields[part.FormName()] = string(val)
			}
		}

		if !sawFile {
			t.Error("no file part found in multipart request")
		}
		if fields["crop_x"] != "0" {
			t.Errorf("crop_x = %q, want 0", fields["crop_x"])
		}

		w.Write([]byte(`{"ok":true}`))
	})

	c := testClient(t, handler)
	resp, err := c.APIMultipart(context.Background(), "users.setPhoto",
		map[string]string{"crop_x": "0"},
		FileParam{
			Fieldname: "image",
			Filename:  "photo.jpg",
			Reader:    strings.NewReader("fake-image-data"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(resp) != `{"ok":true}` {
		t.Errorf("response = %q, want {\"ok\":true}", resp)
	}
}

func TestDoWithRetry_429(t *testing.T) {
	attempts := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			w.Write([]byte(`{"ok":false,"error":"ratelimited"}`))
			return
		}
		w.Write([]byte(`{"ok":true}`))
	})

	c := testClient(t, handler)
	resp, err := c.API(context.Background(), "GET", "users.list", nil, []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if string(resp) != `{"ok":true}` {
		t.Errorf("response = %q, want {\"ok\":true}", resp)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestDoWithRetry_429_NoHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"ok":false}`))
	})

	c := testClient(t, handler)
	_, err := c.API(context.Background(), "GET", "test.method", nil, []byte("{}"))
	if err == nil {
		t.Fatal("expected error for 429 without Retry-After")
	}
	if !strings.Contains(err.Error(), "Retry-After") {
		t.Errorf("error = %q, want mention of Retry-After", err)
	}
}

func TestDoWithRetry_ServerError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`internal server error`))
	})

	c := testClient(t, handler)
	_, err := c.API(context.Background(), "GET", "test.method", nil, []byte("{}"))
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want mention of 500", err)
	}
}

func TestDoWithRetry_SendsCookies(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := r.Cookie("d")
		if err != nil {
			t.Fatalf("missing cookie 'd': %v", err)
		}
		if d.Value != "test-cookie-value" {
			t.Errorf("cookie 'd' = %q, want test-cookie-value", d.Value)
		}
		w.Write([]byte(`{"ok":true}`))
	})

	c := testClient(t, handler)
	c.auth.Cookies = map[string]string{"d": "test-cookie-value"}

	_, err := c.API(context.Background(), "GET", "test.method", nil, []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestAPIMultipart_RetriesOn429(t *testing.T) {
	attempts := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			w.Write([]byte(`{"ok":false,"error":"ratelimited"}`))
			return
		}
		// Verify the multipart body survived the retry.
		ct := r.Header.Get("Content-Type")
		_, mpParams, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatalf("parsing Content-Type on retry: %v", err)
		}
		reader := multipart.NewReader(r.Body, mpParams["boundary"])
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("reading multipart on retry: %v", err)
		}
		data, _ := io.ReadAll(part)
		if string(data) != "retry-data" {
			t.Errorf("file data on retry = %q, want retry-data", data)
		}
		w.Write([]byte(`{"ok":true}`))
	})

	c := testClient(t, handler)
	resp, err := c.APIMultipart(context.Background(), "files.upload",
		nil,
		FileParam{
			Fieldname: "file",
			Filename:  "test.txt",
			Reader:    strings.NewReader("retry-data"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(resp) != `{"ok":true}` {
		t.Errorf("response = %q, want {\"ok\":true}", resp)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}
