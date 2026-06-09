package dingtalkbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	dingtalkAPIBase  = "https://api.dingtalk.com"
	dingtalkOAPIBase = "https://oapi.dingtalk.com"
)

type tokenCache struct {
	mu    sync.Mutex
	token string
	expAt time.Time
}

type tokenManager struct {
	api  tokenCache
	oapi tokenCache
}

func newTokenManager() *tokenManager {
	return &tokenManager{}
}

func (m *tokenManager) getAPIAccessToken(ctx context.Context, appKey, appSecret string) (string, error) {
	m.api.mu.Lock()
	defer m.api.mu.Unlock()
	if m.api.token != "" && time.Now().Before(m.api.expAt) {
		return m.api.token, nil
	}
	body, _ := json.Marshal(map[string]string{
		"appKey":    appKey,
		"appSecret": appSecret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dingtalkAPIBase+"/v1.0/oauth2/accessToken", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("dingtalk access token http %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("dingtalk access token empty")
	}
	expireSec := out.ExpireIn
	if expireSec <= 0 {
		expireSec = 7200
	}
	m.api.token = out.AccessToken
	m.api.expAt = time.Now().Add(time.Duration(expireSec-120) * time.Second)
	return m.api.token, nil
}

func (m *tokenManager) getOAPIAccessToken(ctx context.Context, appKey, appSecret string) (string, error) {
	m.oapi.mu.Lock()
	defer m.oapi.mu.Unlock()
	if m.oapi.token != "" && time.Now().Before(m.oapi.expAt) {
		return m.oapi.token, nil
	}
	url := fmt.Sprintf("%s/gettoken?appkey=%s&appsecret=%s", dingtalkOAPIBase, appKey, appSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.ErrCode != 0 || out.AccessToken == "" {
		return "", fmt.Errorf("dingtalk oapi token errcode=%d msg=%s", out.ErrCode, out.ErrMsg)
	}
	expireSec := out.ExpiresIn
	if expireSec <= 0 {
		expireSec = 7200
	}
	m.oapi.token = out.AccessToken
	m.oapi.expAt = time.Now().Add(time.Duration(expireSec-120) * time.Second)
	return m.oapi.token, nil
}
