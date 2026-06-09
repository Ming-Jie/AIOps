package immanager

import (
	"context"

	"github.com/fisk086/aiops/internal/agent"
	"github.com/fisk086/aiops/internal/dingtalkbot"
	"github.com/fisk086/aiops/internal/larkbot"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/schema"
	"github.com/fisk086/aiops/internal/storage"
	"github.com/fisk086/aiops/internal/telegrambot"
)

type IMManager struct {
	larkClient      *larkbot.Client
	telegramClient  *telegrambot.Client
	dingtalkClient  *dingtalkbot.Client
}

func NewIMManager() *IMManager {
	return &IMManager{
		larkClient:     larkbot.Global(),
		telegramClient: telegrambot.Global(),
		dingtalkClient: dingtalkbot.Global(),
	}
}

func (m *IMManager) RegisterAgent(agent *schema.AgentWithRuntime, runtime *agent.Runtime) error {
	if agent == nil || agent.RuntimeProfile == nil {
		return nil
	}

	enabled := agent.RuntimeProfile.IMEnabled
	if enabled != "lark" {
		m.unregisterLarkBot(agent.ID)
	}
	if enabled != "telegram" {
		m.unregisterTelegramBot(agent.ID)
	}
	if enabled != "dingtalk" {
		m.unregisterDingtalkBot(agent.ID)
	}

	switch enabled {
	case "lark":
		// Always register credentials in the Lark client when IM is configured so manual WS start works.
		// LarkRegisterLongConnection (ws_enabled) only controls NoAutoStartWS / global Start() — see registerLarkBot.
		return m.registerLarkBot(agent, runtime)
	case "telegram":
		return m.registerTelegramBot(agent, runtime)
	case "dingtalk":
		return m.registerDingtalkBot(agent, runtime)
	default:
		return nil
	}
}

func (m *IMManager) UnregisterAgent(agentID int64) {
	m.unregisterLarkBot(agentID)
	m.unregisterTelegramBot(agentID)
	m.unregisterDingtalkBot(agentID)
}

func (m *IMManager) StartAll(ctx context.Context) {
	if m.larkClient.GetBotCount() > 0 {
		go func() {
			m.larkClient.Start(ctx)
		}()
		logger.Info("im manager: larkbot started", "bot_count", m.larkClient.GetBotCount())
	}
	if m.telegramClient.GetBotCount() > 0 {
		go func() {
			m.telegramClient.Start(ctx)
		}()
		logger.Info("im manager: telegrambot started", "bot_count", m.telegramClient.GetBotCount())
	}
	if m.dingtalkClient.GetBotCount() > 0 {
		go func() {
			m.dingtalkClient.Start(ctx)
		}()
		logger.Info("im manager: dingtalkbot started", "bot_count", m.dingtalkClient.GetBotCount())
	}
}

func (m *IMManager) StopAll() {
	m.larkClient.Stop()
	m.telegramClient.Stop()
	m.dingtalkClient.Stop()
	logger.Info("im manager: all im services stopped")
}

func (m *IMManager) GetLarkClient() *larkbot.Client {
	return m.larkClient
}

func (m *IMManager) GetTelegramClient() *telegrambot.Client {
	return m.telegramClient
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
		NoAutoStartWS: !imConfig.LarkRegisterLongConnection(),
		AllowedUsers:  imConfig.AllowedUsers,
	}

	if err := m.larkClient.RegisterBot(botCfg, runtime); err != nil {
		existing := m.larkClient.GetBotConfig(imConfig.AppID)
		if existing != nil && existing.AgentID == agent.ID {
			if err2 := m.larkClient.UpdateBotConfig(botCfg); err2 != nil {
				logger.Warn("im manager: failed to update lark bot", "agent_id", agent.ID, "err", err2)
				return err2
			}
			logger.Info("im manager: lark bot config updated", "agent_id", agent.ID, "app_id", imConfig.AppID)
			if imConfig.LarkRegisterLongConnection() {
				if err := m.larkClient.StartBot(context.Background(), imConfig.AppID); err != nil {
					logger.Warn("im manager: lark ws auto-start failed", "agent_id", agent.ID, "app_id", imConfig.AppID, "err", err)
				}
			}
			return nil
		}
		logger.Warn("im manager: failed to register lark bot", "agent_id", agent.ID, "err", err)
		return err
	}

	logger.Info("im manager: lark bot registered", "agent_id", agent.ID, "app_id", imConfig.AppID, "open_domain", botCfg.OpenAPIDomain)
	if imConfig.LarkRegisterLongConnection() {
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
		WsEnabled:      imConfig.LarkRegisterLongConnection(),
		AllowedUsers:   imConfig.AllowedUsers,
	}

	if err := m.telegramClient.RegisterBot(botCfg, runtime); err != nil {
		existing := m.telegramClient.GetBotConfig(imConfig.TelegramToken)
		if existing != nil && existing.AgentID == agent.ID {
			if err2 := m.telegramClient.UpdateBotConfig(botCfg); err2 != nil {
				logger.Warn("im manager: failed to update telegram bot", "agent_id", agent.ID, "err", err2)
				return err2
			}
			logger.Info("im manager: telegram bot config updated", "agent_id", agent.ID)
			if imConfig.LarkRegisterLongConnection() {
				if err := m.telegramClient.StartBot(context.Background(), imConfig.TelegramToken); err != nil {
					logger.Warn("im manager: telegram auto-start failed", "agent_id", agent.ID, "err", err)
				}
			}
			return nil
		}
		logger.Warn("im manager: failed to register telegram bot", "agent_id", agent.ID, "err", err)
		return err
	}

	logger.Info("im manager: telegram bot registered", "agent_id", agent.ID)
	if imConfig.LarkRegisterLongConnection() {
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
		NoAutoStartWS:   !imConfig.LarkRegisterLongConnection(),
		AllowedUsers:    imConfig.AllowedUsers,
		RequireMention: true,
	}

	if err := m.dingtalkClient.RegisterBot(botCfg, runtime); err != nil {
		existing := m.dingtalkClient.GetBotConfig(imConfig.AppID)
		if existing != nil && existing.AgentID == agent.ID {
			if err2 := m.dingtalkClient.UpdateBotConfig(botCfg); err2 != nil {
				logger.Warn("im manager: failed to update dingtalk bot", "agent_id", agent.ID, "err", err2)
				return err2
			}
			logger.Info("im manager: dingtalk bot config updated", "agent_id", agent.ID, "app_id", imConfig.AppID)
			if imConfig.LarkRegisterLongConnection() {
				if err := m.dingtalkClient.StartBot(context.Background(), imConfig.AppID); err != nil {
					logger.Warn("im manager: dingtalk stream auto-start failed", "agent_id", agent.ID, "app_id", imConfig.AppID, "err", err)
				}
			}
			return nil
		}
		logger.Warn("im manager: failed to register dingtalk bot", "agent_id", agent.ID, "err", err)
		return err
	}

	logger.Info("im manager: dingtalk bot registered", "agent_id", agent.ID, "app_id", imConfig.AppID)
	if imConfig.LarkRegisterLongConnection() {
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
