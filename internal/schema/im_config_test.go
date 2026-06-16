package schema

import "testing"

func TestIMConfig_IMWsAutoStartEnabled(t *testing.T) {
	t.Run("nil defaults to false", func(t *testing.T) {
		if (IMConfig{}).IMWsAutoStartEnabled() {
			t.Fatal("expected false when ws_enabled unset")
		}
	})
	t.Run("explicit false", func(t *testing.T) {
		v := false
		if (IMConfig{WsEnabled: &v}).IMWsAutoStartEnabled() {
			t.Fatal("expected false")
		}
	})
	t.Run("explicit true", func(t *testing.T) {
		v := true
		if !(IMConfig{WsEnabled: &v}).IMWsAutoStartEnabled() {
			t.Fatal("expected true")
		}
	})
}

func TestIMConfig_LarkRegisterLongConnectionAlias(t *testing.T) {
	v := true
	cfg := IMConfig{WsEnabled: &v}
	if !cfg.LarkRegisterLongConnection() || !cfg.IMWsAutoStartEnabled() {
		t.Fatal("alias should match IMWsAutoStartEnabled")
	}
}
