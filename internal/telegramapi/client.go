// Package telegramapi builds the Telegram Bot API client.
//
// In networks where api.telegram.org is blocked (for example in Russia) the bot
// must go through a proxy. Only HTTP(S) proxies are supported: the Bot API is a
// plain HTTPS API, so an HTTP CONNECT proxy is enough and no extra dependency
// is required.
package telegramapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"mega-trainer-go/internal/config"
)

// requestTimeout is long enough for file uploads but still bounds hung requests.
const requestTimeout = 60 * time.Second

// NewHTTPClient builds the HTTP client used for Bot API calls. When proxyURL is
// empty the default system proxy settings are used.
func NewHTTPClient(proxyURL string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("parse telegram proxy URL: %w", err)
		}
		if parsed.Host == "" {
			return nil, fmt.Errorf("parse telegram proxy URL %q: host is required", proxyURL)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, fmt.Errorf("unsupported telegram proxy scheme %q: use http://host:port", parsed.Scheme)
		}

		transport.Proxy = http.ProxyURL(parsed)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   requestTimeout,
	}, nil
}

// New creates an authorized Bot API client and switches it to long polling by
// deleting any webhook. Telegram replies 409 Conflict to getUpdates while a
// webhook is active, which looks like a silently frozen bot.
func New(cfg config.TelegramConfig) (*tgbotapi.BotAPI, error) {
	client, err := NewHTTPClient(cfg.ProxyURL)
	if err != nil {
		return nil, err
	}

	api, err := tgbotapi.NewBotAPIWithClient(cfg.BotToken, apiEndpoint(cfg.APIURL), client)
	if err != nil {
		return nil, fmt.Errorf(
			"authorize telegram bot (if api.telegram.org is blocked, set TELEGRAM_PROXY_URL, e.g. http://127.0.0.1:12334): %w",
			err,
		)
	}
	api.Debug = cfg.Debug

	if _, err := api.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: false}); err != nil {
		return nil, fmt.Errorf("delete telegram webhook: %w", err)
	}

	return api, nil
}

// apiEndpoint returns the Bot API URL template. The public endpoint is used
// unless an explicit override is configured (used by end-to-end tests).
func apiEndpoint(override string) string {
	if override == "" {
		return tgbotapi.APIEndpoint
	}
	return strings.TrimSuffix(override, "/") + "/bot%s/%s"
}

// ProxyHost returns the proxy host without credentials, so it can be logged.
// An empty string means that no proxy is configured.
func ProxyHost(proxyURL string) string {
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return ""
	}
	return parsed.Host
}
