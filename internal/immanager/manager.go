package immanager

import (
	"context"
	"fmt"
	"strings"

	"github.com/fisk086/aiops/internal/agent"
	"github.com/fisk086/aiops/internal/dingtalkbot"
	"github.com/fisk086/aiops/internal/larkbot"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/qqbot"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/storage"
	"github.com/fisk086/aiops/internal/telegrambot"
)

type IMManager struct {
	larkClient     *larkbot.Client
	telegramClient *telegrambot.Client
	dingtalkClient *dingtalkbot.Client
	qqClient       *qqbot.Client
	enabledTypes   map[string]bool
}

var allIMTypes = []string{"lark", "telegram", "dingtalk", "qq"}

func NewIMManager(imTypes string) *IMManager {
	enabled := parseEnabledTypes(imTypes)
	return &IMManager{
		larkClient:     larkbot.Global(),
		telegramClient: telegrambot.Global(),
		dingtalkClient: dingtalkbot.Global(),
		qqClient:       qqbot.Global(),
		enabledTypes:   enabled,
	}
}

func parseEnabledTypes(imTypes string) map[string]bool {
	enabled := make(map[string]bool)
	trimmed := strings.TrimSpace(imTypes)
	if trimmed == "" {
		for _, t := range allIMTypes {
			enabled[t] = true
		}
		return enabled
	}
	for _, t := range strings.Split(trimmed, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			enabled[t] = true
		}
	}
	return enabled
}

func (m *IMManager) EnabledTypes() []string {
	var types []string
	for _, t := range allIMTypes {
		if m.enabledTypes[t] {
			types = append(types, t)
		}
	}
	return types
}

func (m *IMManager) IsTypeEnabled(imType string) bool {
	return m.enabledTypes[imType]
}

func (m *IMManager) RegisterAgent(agent *schema.AgentWithRuntime, runtime *agent.Runtime) error {
	if agent == nil || agent.RuntimeProfile == nil {
		return nil
	}

	enabled := agent.RuntimeProfile.IMEnabled
	if enabled == "" {
		return nil
	}

	if !m.enabledTypes[enabled] {
		logger.Info("im manager: skipping disabled im type",
			"agent_id", agent.ID, "im_type", enabled)
		return nil
	}

	if enabled != "lark" {
		m.unregisterLarkBot(agent.ID)
	}
	if enabled != "telegram" {
		m.unregisterTelegramBot(agent.ID)
	}
	if enabled != "dingtalk" {
		m.unregisterDingtalkBot(agent.ID)
	}
	if enabled != "qq" {
		m.unregisterQQBot(agent.ID)
	}

	switch enabled {
	case "lark":
		return m.registerLarkBot(agent, runtime)
	case "telegram":
		return m.registerTelegramBot(agent, runtime)
	case "dingtalk":
		return m.registerDingtalkBot(agent, runtime)
	case "qq":
		return m.registerQQBot(agent, runtime)
	default:
		return nil
	}
}

func (m *IMManager) UnregisterAgent(agentID int64) {
	m.unregisterLarkBot(agentID)
	m.unregisterTelegramBot(agentID)
	m.unregisterDingtalkBot(agentID)
	m.unregisterQQBot(agentID)
}

func (m *IMManager) StartAll(ctx context.Context) {
	startBot := func(name string, fn func(context.Context)) {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("im manager: bot panic", "bot", name, "recover", r)
				}
			}()
			fn(ctx)
		}()
	}
	if m.enabledTypes["lark"] && m.larkClient.GetBotCount() > 0 {
		startBot("lark", m.larkClient.Start)
		logger.Info("im manager: larkbot started", "bot_count", m.larkClient.GetBotCount())
	}
	if m.enabledTypes["telegram"] && m.telegramClient.GetBotCount() > 0 {
		startBot("telegram", m.telegramClient.Start)
		logger.Info("im manager: telegrambot started", "bot_count", m.telegramClient.GetBotCount())
	}
	if m.enabledTypes["dingtalk"] && m.dingtalkClient.GetBotCount() > 0 {
		startBot("dingtalk", m.dingtalkClient.Start)
		logger.Info("im manager: dingtalkbot started", "bot_count", m.dingtalkClient.GetBotCount())
	}
	if m.enabledTypes["qq"] && m.qqClient.GetBotCount() > 0 {
		startBot("qq", m.qqClient.Start)
		logger.Info("im manager: qqbot started", "bot_count", m.qqClient.GetBotCount())
	}
}

func (m *IMManager) StopAll() {
	m.larkClient.Stop()
	m.telegramClient.Stop()
	m.dingtalkClient.Stop()
	m.qqClient.Stop()
	logger.Info("im manager: all im services stopped")
}

func (m *IMManager) GetLarkClient() *larkbot.Client {
	return m.larkClient
}

func (m *IMManager) GetTelegramClient() *telegrambot.Client {
	return m.telegramClient
}

func (m *IMManager) GetDingtalkClient() *dingtalkbot.Client {
	return m.dingtalkClient
}

func (m *IMManager) GetQQClient() *qqbot.Client {
	return m.qqClient
}

func (m *IMManager) ScanAndRegister(store storage.Storage, runtime *agent.Runtime) error {
	agents, err := store.ListAgents()
	if err != nil {
		return err
	}

	var registeredCount int
	for _, a := range agents {
		if a == nil {
			continue
		}
		full, err := store.GetAgent(a.ID)
		if err != nil || full == nil {
			continue
		}

		if err := m.RegisterAgent(full, runtime); err != nil {
			logger.Warn("im manager: failed to register bot for agent", "agent_id", a.ID, "err", err)
		} else {
			registeredCount++
		}
	}

	logger.Info("im manager: scan complete", "registered_count", registeredCount)
	return nil
}

func (m *IMManager) registerLarkBot(agent *schema.AgentWithRuntime, runtime *agent.Runtime) error {
	imConfig := agent.RuntimeProfile.IMConfig
	if imConfig.AppID == "" || imConfig.AppSecret == "" {
		logger.Warn("im manager: lark bot config incomplete", "agent_id", agent.ID)
		return nil
	}

	botCfg := &larkbot.BotConfig{
		AppID:         imConfig.AppID,
		AppSecret:     imConfig.AppSecret,
		AgentID:       agent.ID,
		InvokeTimeout: 120,
		OpenAPIDomain: larkbot.OpenAPIDomainFromIMConfig(imConfig),
		NoAutoStartWS: !imConfig.IMWsAutoStartEnabled(),
		AllowedUsers:  imConfig.AllowedUsers,
	}

	// Bot already registered — update config only, never touch the stream client.
	if existing := m.larkClient.GetBotConfig(imConfig.AppID); existing != nil {
		if existing.AgentID != agent.ID {
			logger.Warn("im manager: lark app_id already bound to another agent", "app_id", imConfig.AppID, "agent_id", agent.ID)
			return fmt.Errorf("app_id %s already bound to agent %d", imConfig.AppID, existing.AgentID)
		}
		if err := m.larkClient.UpdateBotConfig(botCfg); err != nil {
			logger.Warn("im manager: failed to update lark bot", "agent_id", agent.ID, "err", err)
			return err
		}
		logger.Info("im manager: lark bot config updated", "agent_id", agent.ID, "app_id", imConfig.AppID)
		return nil
	}

	if err := m.larkClient.RegisterBot(botCfg, runtime); err != nil {
		logger.Warn("im manager: failed to register lark bot", "agent_id", agent.ID, "err", err)
		return err
	}
	logger.Info("im manager: lark bot registered", "agent_id", agent.ID, "app_id", imConfig.AppID, "open_domain", botCfg.OpenAPIDomain)
	if imConfig.IMWsAutoStartEnabled() {
		if err := m.larkClient.StartBot(context.Background(), imConfig.AppID); err != nil {
			logger.Warn("im manager: lark ws auto-start failed", "agent_id", agent.ID, "app_id", imConfig.AppID, "err", err)
		}
	}
	return nil
}

func (m *IMManager) unregisterLarkBot(agentID int64) {
	bots := m.larkClient.GetBots()
	for appID, entry := range bots {
		if entry.Config.AgentID == agentID {
			m.larkClient.UnregisterBot(appID)
			logger.Info("im manager: lark bot unregistered", "agent_id", agentID, "app_id", appID)
			break
		}
	}
}

func (m *IMManager) registerTelegramBot(agent *schema.AgentWithRuntime, runtime *agent.Runtime) error {
	imConfig := agent.RuntimeProfile.IMConfig
	if imConfig.TelegramToken == "" {
		logger.Warn("im manager: telegram bot config incomplete", "agent_id", agent.ID)
		return nil
	}

	botCfg := &telegrambot.BotConfig{
		Token:          imConfig.TelegramToken,
		ChatID:         imConfig.TelegramChatID,
		AgentID:        agent.ID,
		InvokeTimeout:  120,
		WebhookEnabled: imConfig.WebhookURL != "",
		WebhookURL:     imConfig.WebhookURL,
		WsEnabled:      imConfig.IMWsAutoStartEnabled(),
		AllowedUsers:   imConfig.AllowedUsers,
	}

	// Bot already registered — update config only, never touch the poll/stream.
	if existing := m.telegramClient.GetBotConfig(imConfig.TelegramToken); existing != nil {
		if existing.AgentID != agent.ID {
			logger.Warn("im manager: telegram token already bound to another agent", "agent_id", agent.ID)
			return fmt.Errorf("telegram token already bound to agent %d", existing.AgentID)
		}
		if err := m.telegramClient.UpdateBotConfig(botCfg); err != nil {
			logger.Warn("im manager: failed to update telegram bot", "agent_id", agent.ID, "err", err)
			return err
		}
		logger.Info("im manager: telegram bot config updated", "agent_id", agent.ID)
		return nil
	}

	if err := m.telegramClient.RegisterBot(botCfg, runtime); err != nil {
		logger.Warn("im manager: failed to register telegram bot", "agent_id", agent.ID, "err", err)
		return err
	}
	logger.Info("im manager: telegram bot registered", "agent_id", agent.ID)
	if imConfig.IMWsAutoStartEnabled() {
		if err := m.telegramClient.StartBot(context.Background(), imConfig.TelegramToken); err != nil {
			logger.Warn("im manager: telegram auto-start failed", "agent_id", agent.ID, "err", err)
		}
	}
	return nil
}

func (m *IMManager) unregisterTelegramBot(agentID int64) {
	bots := m.telegramClient.GetBots()
	for token, entry := range bots {
		if entry.Config.AgentID == agentID {
			m.telegramClient.UnregisterBot(token)
			logger.Info("im manager: telegram bot unregistered", "agent_id", agentID, "token_prefix", token[:8])
			break
		}
	}
}

func (m *IMManager) registerDingtalkBot(agent *schema.AgentWithRuntime, runtime *agent.Runtime) error {
	imConfig := agent.RuntimeProfile.IMConfig
	if imConfig.AppID == "" || imConfig.AppSecret == "" {
		logger.Warn("im manager: dingtalk bot config incomplete", "agent_id", agent.ID)
		return nil
	}

	botCfg := &dingtalkbot.BotConfig{
		AppID:           imConfig.AppID,
		AppSecret:       imConfig.AppSecret,
		AgentID:         agent.ID,
		InvokeTimeout:   120,
		NoAutoStartWS:   !imConfig.IMWsAutoStartEnabled(),
		AllowedUsers:    imConfig.AllowedUsers,
		RequireMention: true,
	}

	// Bot already registered — update config only, never touch the stream client.
	if existing := m.dingtalkClient.GetBotConfig(imConfig.AppID); existing != nil {
		if existing.AgentID != agent.ID {
			logger.Warn("im manager: dingtalk app_id already bound to another agent", "app_id", imConfig.AppID, "agent_id", agent.ID)
			return fmt.Errorf("app_id %s already bound to agent %d", imConfig.AppID, existing.AgentID)
		}
		if err := m.dingtalkClient.UpdateBotConfig(botCfg); err != nil {
			logger.Warn("im manager: failed to update dingtalk bot", "agent_id", agent.ID, "err", err)
			return err
		}
		logger.Info("im manager: dingtalk bot config updated", "agent_id", agent.ID, "app_id", imConfig.AppID)
		return nil
	}

	if err := m.dingtalkClient.RegisterBot(botCfg, runtime); err != nil {
		logger.Warn("im manager: failed to register dingtalk bot", "agent_id", agent.ID, "err", err)
		return err
	}
	logger.Info("im manager: dingtalk bot registered", "agent_id", agent.ID, "app_id", imConfig.AppID)
	if imConfig.IMWsAutoStartEnabled() {
		if err := m.dingtalkClient.StartBot(context.Background(), imConfig.AppID); err != nil {
			logger.Warn("im manager: dingtalk stream auto-start failed", "agent_id", agent.ID, "app_id", imConfig.AppID, "err", err)
		}
	}
	return nil
}

func (m *IMManager) unregisterDingtalkBot(agentID int64) {
	bots := m.dingtalkClient.GetBots()
	for appID, entry := range bots {
		if entry.Config.AgentID == agentID {
			m.dingtalkClient.UnregisterBot(appID)
			logger.Info("im manager: dingtalk bot unregistered", "agent_id", agentID, "app_id", appID)
			break
		}
	}
}

func (m *IMManager) registerQQBot(agent *schema.AgentWithRuntime, runtime *agent.Runtime) error {
	imConfig := agent.RuntimeProfile.IMConfig
	if imConfig.QQAppID == "" || imConfig.QQBotToken == "" {
		logger.Warn("im manager: qq bot config incomplete", "agent_id", agent.ID)
		return nil
	}

	botCfg := &qqbot.BotConfig{
		AppID:         imConfig.QQAppID,
		BotToken:      imConfig.QQBotToken,
		AgentID:       agent.ID,
		InvokeTimeout: 120,
		NoAutoStartWS: !imConfig.IMWsAutoStartEnabled(),
		AllowedUsers:  imConfig.AllowedUsers,
	}

	// Bot already registered — update config only, never touch the stream client.
	if existing := m.qqClient.GetBotConfig(imConfig.QQAppID); existing != nil {
		if existing.AgentID != agent.ID {
			logger.Warn("im manager: qq app_id already bound to another agent", "app_id", imConfig.QQAppID, "agent_id", agent.ID)
			return fmt.Errorf("qq app_id %s already bound to agent %d", imConfig.QQAppID, existing.AgentID)
		}
		if err := m.qqClient.UpdateBotConfig(botCfg); err != nil {
			logger.Warn("im manager: failed to update qq bot", "agent_id", agent.ID, "err", err)
			return err
		}
		logger.Info("im manager: qq bot config updated", "agent_id", agent.ID, "app_id", imConfig.QQAppID)
		return nil
	}

	if err := m.qqClient.RegisterBot(botCfg, runtime); err != nil {
		logger.Warn("im manager: failed to register qq bot", "agent_id", agent.ID, "err", err)
		return err
	}
	logger.Info("im manager: qq bot registered", "agent_id", agent.ID, "app_id", imConfig.QQAppID)
	if imConfig.IMWsAutoStartEnabled() {
		if err := m.qqClient.StartBot(context.Background(), imConfig.QQAppID); err != nil {
			logger.Warn("im manager: qq ws auto-start failed", "agent_id", agent.ID, "app_id", imConfig.QQAppID, "err", err)
		}
	}
	return nil
}

func (m *IMManager) unregisterQQBot(agentID int64) {
	bots := m.qqClient.GetBots()
	for appID, entry := range bots {
		if entry.Config.AgentID == agentID {
			m.qqClient.UnregisterBot(appID)
			logger.Info("im manager: qq bot unregistered", "agent_id", agentID, "app_id", appID)
			break
		}
	}
}
