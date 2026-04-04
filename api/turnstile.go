package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type turnstileVerifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

// trustProxy returns true when TRUST_PROXY is set (e.g. 1, true, yes).
// Only then do we use X-Forwarded-For for Turnstile's remoteip (avoids client spoofing when not behind a trusted proxy).
func trustProxy() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("TRUST_PROXY")))
	return v == "1" || v == "true" || v == "yes"
}

// turnstileClientIP is the IP sent to Cloudflare Turnstile siteverify.
// Uses the direct TCP remote address unless TRUST_PROXY is set, then the first X-Forwarded-For hop.
func turnstileClientIP(r *http.Request) string {
	if trustProxy() {
		return requestRemoteAddr(r)
	}
	return clientIP(r)
}

func verifyTurnstile(ctx context.Context, secret, token, remoteIP string) error {
	if strings.TrimSpace(secret) == "" {
		return nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("missing turnstile token")
	}

	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(
		reqCtx,
		http.MethodPost,
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("turnstile request: %w", err)
	}
	defer resp.Body.Close()

	var out turnstileVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("turnstile decode: %w", err)
	}
	if !out.Success {
		return fmt.Errorf("turnstile rejected: %v", out.ErrorCodes)
	}
	return nil
}
