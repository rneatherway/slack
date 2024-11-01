package config

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
)

var stmt = "SELECT value, encrypted_value FROM cookies WHERE host_key=\".slack.com\" AND name=\"d\""

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

	fmt.Printf("encryptedValue: %v\n", encryptedValue)

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

	return string(decryptedValue), err
}
