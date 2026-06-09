package larkbot

import (
	"testing"
	"time"

	"github.com/larksuite/oapi-sdk-go/v3/channel/safety"
)

func TestMessageDedup(t *testing.T) {
	cache := safety.NewDedupCache(100, time.Hour)
	if cache.IsDuplicate("om_test_1") {
		t.Fatal("first seen should not be duplicate")
	}
	if !cache.IsDuplicate("om_test_1") {
		t.Fatal("second seen should be duplicate")
	}
}
