package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// newFallbackTestToken 插入一条用于「备用模型兜底」相关测试的令牌，并在测试结束后物理删除。
// 依赖同包 TestMain 已迁移好的共享 SQLite 内存库。
func newFallbackTestToken(t *testing.T, key string, enabled bool) *Token {
	t.Helper()

	token := &Token{
		UserId:               1,
		Name:                 "fallback-test-" + key,
		Key:                  key,
		Status:               common.TokenStatusEnabled,
		CreatedTime:          common.GetTimestamp(),
		AccessedTime:         common.GetTimestamp(),
		ExpiredTime:          -1,
		RemainQuota:          100,
		UnlimitedQuota:       true,
		FallbackModelEnabled: enabled,
	}
	require.NoError(t, DB.Create(token).Error)
	t.Cleanup(func() {
		DB.Unscoped().Delete(&Token{}, token.Id)
	})
	return token
}

// TestTokenUpdatePersistsFallbackModelEnabled 验证 Token.Update() 的 Select 列表包含
// fallback_model_enabled，否则「开启 -> 关闭」的更新会因为 GORM 跳过零值而不落库。
func TestTokenUpdatePersistsFallbackModelEnabled(t *testing.T) {
	require.True(t, DB.Migrator().HasColumn(&Token{}, "fallback_model_enabled"),
		"tokens 表应包含 fallback_model_enabled 列")

	token := newFallbackTestToken(t, "sk-fallback-update-happy-path", true)

	var loaded Token
	require.NoError(t, DB.First(&loaded, token.Id).Error)
	require.True(t, loaded.FallbackModelEnabled, "Insert 应写入 fallback_model_enabled=true")

	// true -> false：若 Update() 的 Select 未包含该字段，GORM 会跳过零值，DB 中仍为 true。
	loaded.FallbackModelEnabled = false
	loaded.Name = "fallback-test-renamed"
	require.NoError(t, loaded.Update())

	var afterDisable Token
	require.NoError(t, DB.First(&afterDisable, token.Id).Error)
	require.False(t, afterDisable.FallbackModelEnabled,
		"关闭兜底后必须落库为 false（Select 列表缺少 fallback_model_enabled？）")
	require.Equal(t, "fallback-test-renamed", afterDisable.Name)

	// false -> true：反向也要能落库。
	afterDisable.FallbackModelEnabled = true
	require.NoError(t, afterDisable.Update())

	var afterEnable Token
	require.NoError(t, DB.First(&afterEnable, token.Id).Error)
	require.True(t, afterEnable.FallbackModelEnabled, "重新开启兜底后必须落库为 true")
}

// TestTokenInsertDefaultsFallbackModelEnabledToFalse 验证新建令牌默认关闭兜底。
func TestTokenInsertDefaultsFallbackModelEnabledToFalse(t *testing.T) {
	token := &Token{
		UserId:         1,
		Name:           "fallback-test-default",
		Key:            "sk-fallback-default",
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
	}
	require.NoError(t, token.Insert())
	t.Cleanup(func() {
		DB.Unscoped().Delete(&Token{}, token.Id)
	})

	var loaded Token
	require.NoError(t, DB.First(&loaded, token.Id).Error)
	require.False(t, loaded.FallbackModelEnabled)
}

// TestEnsureTokenFallbackModelEnabledColumn_Idempotent 验证升级迁移函数在列已存在时可直接返回。
func TestEnsureTokenFallbackModelEnabledColumn_Idempotent(t *testing.T) {
	require.NoError(t, ensureTokenFallbackModelEnabledColumn())
	require.True(t, DB.Migrator().HasColumn(&Token{}, "fallback_model_enabled"))
}
