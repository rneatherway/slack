package config

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
)

var (
	stmt = `SELECT value, encrypted_value FROM cookies WHERE host_key=".slack.com" AND name="d"`

	// Chromium prefixes the encrypted value with a SHA256 hash of the domain
	// name. Once the value is decrypted, we must remove the hash to get the
	// cookie value back.
	// See https://chromium-review.googlesource.com/c/chromium/src/+/5792044
	prefixes = [][]byte{
		// slack.com
		{3, 202, 236, 172, 132, 247, 212, 240, 217, 211, 68, 226, 103, 153, 245, 64, 85, 68, 2, 183, 83, 182, 186, 218, 14, 102, 237, 62, 231, 241, 231, 142},
		// .slack.com
		{145, 28, 115, 68, 173, 92, 42, 78, 104, 243, 5, 63, 24, 206, 51, 190, 31, 169, 160, 244, 247, 106, 147, 228, 60, 68, 92, 134, 105, 199, 162, 120},
	}
)

type CookieDecryptor interface {
	Decrypt(value, key []byte) ([]byte, error)
}

func decrypt(encryptedValue, key []byte) ([]byte, error) {
	switch runtime.GOOS {
	case "windows":
		return WindowsDecryptor{}.Decrypt(encryptedValue, key)
	case "darwin":
		return UnixCookieDecryptor{Rounds: 1003}.Decrypt(encryptedValue, key)
	case "linux":
		return UnixCookieDecryptor{Rounds: 1}.Decrypt(encryptedValue, key)
	default:
		panic(fmt.Sprintf("platform %q not supported", runtime.GOOS))
	}
}

func removeDomainHashPrefix(value []byte) []byte {
	for _, prefix := range prefixes {
		if bytes.HasPrefix(value, prefix) {
			return value[len(prefix):]
		}
	}

	return value
}

func GetCookie() (string, error) {
	cookieDBFile := path.Join(slackConfigDir(), "Cookies")
	if runtime.GOOS == "windows" {
		cookieDBFile = path.Join(slackConfigDir(), "Network", "Cookies")
	}

	stat, err := os.Stat(cookieDBFile)
	if err != nil {
		return "", fmt.Errorf("could not access Slack cookie database: %w", err)
	}
	if stat.IsDir() {
		return "", fmt.Errorf("directory found at expected Slack cookie database location %q", cookieDBFile)
	}

	if cookieDBFile == "" {
		return "", errors.New("no Slack cookie database found. Are you definitely logged in?")
	}

	db, err := sql.Open("sqlite", cookieDBFile)
	if err != nil {
		return "", err
	}

	var cookie string
	var encryptedValue []byte
	err = db.QueryRow(stmt).Scan(&cookie, &encryptedValue)
	if err != nil {
		return "", err
	}

	if cookie != "" {
		return cookie, nil
	}

	// Remove the version number e.g. v11
	encryptedValue = encryptedValue[3:]

	// We need to decrypt the cookie.
	key, err := Password()
	if err != nil {
		return "", fmt.Errorf("failed to get cookie password: %w", err)
	}

	decryptedValue, err := decrypt(encryptedValue, key)
	if err != nil {
		return "", err
	}

	decryptedValue = removeDomainHashPrefix(decryptedValue)

	return string(decryptedValue), err
}
