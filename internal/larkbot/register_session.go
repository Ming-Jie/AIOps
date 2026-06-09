package larkbot

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	larkreg "github.com/larksuite/oapi-sdk-go/v3/scene/registration"
)

type RegisterAppSessionStatus string

const (
	RegisterStatusPending   RegisterAppSessionStatus = "pending"
	RegisterStatusQRReady   RegisterAppSessionStatus = "qr_ready"
	RegisterStatusPolling   RegisterAppSessionStatus = "polling"
	RegisterStatusCompleted RegisterAppSessionStatus = "completed"
	RegisterStatusDenied    RegisterAppSessionStatus = "denied"
	RegisterStatusExpired   RegisterAppSessionStatus = "expired"
	RegisterStatusFailed    RegisterAppSessionStatus = "failed"
	RegisterStatusCancelled RegisterAppSessionStatus = "cancelled"
)

// RegisterAppSession is returned by GET register-app/:sessionId.
type RegisterAppSession struct {
	SessionID    string                   `json:"session_id"`
	Status       RegisterAppSessionStatus `json:"status"`
	QRURL        string                   `json:"qr_url,omitempty"`
	AppID          string                   `json:"app_id,omitempty"`
	AppSecret      string                   `json:"app_secret,omitempty"`
	OperatorOpenID string                   `json:"operator_open_id,omitempty"`
	ChannelBound   bool                     `json:"channel_bound,omitempty"`
	Error        string                   `json:"error,omitempty"`
	Message      string                   `json:"message,omitempty"`
}

// RegisterAppStartOptions configures a QR registration session.
type RegisterAppStartOptions struct {
	AgentID        int64
	AgentName      string
	LarkOpenDomain string
	AppPresetName  string
	Source         string
	// OnCompleted is invoked when Lark returns client credentials (before session status becomes completed).
	OnCompleted func(appID, appSecret, tenantBrand, operatorOpenID string) (channelBound bool, err error)
}

type registerSessionState struct {
	mu      sync.RWMutex
	session RegisterAppSession
	cancel  context.CancelFunc
}

func (s *registerSessionState) snapshot() RegisterAppSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.session
}

func (s *registerSessionState) patch(fn func(*RegisterAppSession)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.session)
}

// RegisterAppSessionManager stores in-flight Lark QR registration sessions.
type RegisterAppSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*registerSessionState
}

func NewRegisterAppSessionManager() *RegisterAppSessionManager {
	return &RegisterAppSessionManager{sessions: make(map[string]*registerSessionState)}
}

func (m *RegisterAppSessionManager) Start(parent context.Context, opts RegisterAppStartOptions) (string, error) {
	if opts.AgentID < 1 {
		return "", errors.New("agent_id is required")
	}
	sessionID := uuid.NewString()
	ctx, cancel := context.WithTimeout(parent, 12*time.Minute)

	state := &registerSessionState{
		session: RegisterAppSession{
			SessionID: sessionID,
			Status:    RegisterStatusPending,
		},
		cancel: cancel,
	}

	m.mu.Lock()
	m.sessions[sessionID] = state
	m.mu.Unlock()

	go m.runSession(ctx, state, opts)
	return sessionID, nil
}

func (m *RegisterAppSessionManager) Get(sessionID string) (RegisterAppSession, bool) {
	m.mu.RLock()
	state, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return RegisterAppSession{}, false
	}
	return state.snapshot(), true
}

func (m *RegisterAppSessionManager) Cancel(sessionID string) bool {
	m.mu.Lock()
	state, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return false
	}
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	if state.cancel != nil {
		state.cancel()
	}
	state.patch(func(s *RegisterAppSession) {
		if s.Status == RegisterStatusCompleted || s.Status == RegisterStatusDenied ||
			s.Status == RegisterStatusExpired || s.Status == RegisterStatusFailed {
			return
		}
		s.Status = RegisterStatusCancelled
		s.Message = "会话已取消"
	})
	return true
}

func (m *RegisterAppSessionManager) runSession(ctx context.Context, state *registerSessionState, opts RegisterAppStartOptions) {
	sessionID := state.session.SessionID
	defer func() {
		if state.cancel != nil {
			state.cancel()
		}
		time.AfterFunc(15*time.Minute, func() {
			m.mu.Lock()
			delete(m.sessions, sessionID)
			m.mu.Unlock()
		})
	}()

	source := strings.TrimSpace(opts.Source)
	if source == "" {
		source = "aiops"
	}
	presetName := strings.TrimSpace(opts.AppPresetName)
	if presetName == "" && strings.TrimSpace(opts.AgentName) != "" {
		presetName = opts.AgentName + " IM"
	}

	regOpts := &larkreg.Options{
		Source: source,
		OnQRCode: func(info *larkreg.QRCodeInfo) {
			if info == nil {
				return
			}
			state.patch(func(s *RegisterAppSession) {
				s.Status = RegisterStatusQRReady
				s.QRURL = info.URL
				s.Message = "请使用飞书 / Lark 扫描二维码并确认授权"
			})
		},
		OnStatusChange: func(info *larkreg.StatusChangeInfo) {
			if info == nil {
				return
			}
			switch info.Status {
			case larkreg.StatusPolling, larkreg.StatusSlowDown, larkreg.StatusDomainSwitched:
				state.patch(func(s *RegisterAppSession) {
					if s.Status == RegisterStatusCompleted {
						return
					}
					s.Status = RegisterStatusPolling
					s.Message = "已扫码，等待你在客户端确认…"
				})
			}
		},
	}
	if presetName != "" {
		regOpts.AppPreset = &larkreg.AppPreset{Name: presetName}
	}
	if d := strings.TrimSpace(opts.LarkOpenDomain); d != "" {
		if strings.Contains(strings.ToLower(d), "larksuite") {
			regOpts.LarkDomain = "https://accounts.larksuite.com"
		} else {
			regOpts.Domain = "https://accounts.feishu.cn"
		}
	}

	result, err := larkreg.RegisterApp(ctx, regOpts)
	if err != nil {
		m.finishWithError(state, err)
		return
	}
	if result == nil || strings.TrimSpace(result.ClientID) == "" || strings.TrimSpace(result.ClientSecret) == "" {
		m.finishWithError(state, &larkreg.RegisterAppError{Code: "invalid_response", Description: "missing client credentials"})
		return
	}

	tenantBrand := ""
	operatorOpenID := ""
	if result.UserInfo != nil {
		tenantBrand = result.UserInfo.TenantBrand
		operatorOpenID = strings.TrimSpace(result.UserInfo.OpenID)
	}

	channelBound := false
	if opts.OnCompleted != nil {
		var bindErr error
		channelBound, bindErr = opts.OnCompleted(result.ClientID, result.ClientSecret, tenantBrand, operatorOpenID)
		if bindErr != nil {
			state.patch(func(s *RegisterAppSession) {
				s.Status = RegisterStatusFailed
				s.AppID = result.ClientID
				s.AppSecret = result.ClientSecret
				s.Error = bindErr.Error()
				s.Message = "应用已创建，但写入智能体 IM 配置失败"
			})
			return
		}
	}

	state.patch(func(s *RegisterAppSession) {
		s.Status = RegisterStatusCompleted
		s.AppID = result.ClientID
		s.AppSecret = result.ClientSecret
		s.OperatorOpenID = operatorOpenID
		s.ChannelBound = channelBound
		s.Message = "应用创建完成"
	})
}

func (m *RegisterAppSessionManager) finishWithError(state *registerSessionState, err error) {
	var denied *larkreg.AccessDeniedError
	var expired *larkreg.ExpiredError
	var regErr *larkreg.RegisterAppError

	switch {
	case errors.As(err, &denied):
		state.patch(func(s *RegisterAppSession) {
			s.Status = RegisterStatusDenied
			s.Error = denied.Error()
			s.Message = "你已拒绝授权"
		})
	case errors.As(err, &expired):
		state.patch(func(s *RegisterAppSession) {
			s.Status = RegisterStatusExpired
			s.Error = expired.Error()
			s.Message = "二维码已过期，请重新发起"
		})
	case errors.Is(err, context.Canceled):
		state.patch(func(s *RegisterAppSession) {
			if s.Status == RegisterStatusCompleted {
				return
			}
			s.Status = RegisterStatusCancelled
			s.Message = "会话已取消"
		})
	default:
		msg := err.Error()
		if errors.As(err, &regErr) {
			msg = regErr.Description
		}
		state.patch(func(s *RegisterAppSession) {
			s.Status = RegisterStatusFailed
			s.Error = msg
			s.Message = "创建失败"
		})
	}
}
