package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newGroupGatingContext(t *testing.T, tokenGroup string, userGroup string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyTokenGroup, tokenGroup)
	common.SetContextKey(c, constant.ContextKeyUserGroup, userGroup)
	return c
}

// TestIsAutoGroupRequest auto 分组判定
func TestIsAutoGroupRequest(t *testing.T) {
	c := newGroupGatingContext(t, "auto", "default")
	require.True(t, IsAutoGroupRequest(c))

	c2 := newGroupGatingContext(t, "vip", "default")
	require.False(t, IsAutoGroupRequest(c2))

	require.False(t, IsAutoGroupRequest(nil))
}

// TestFallbackSearchGroups_SpecificGroupOnly
//
// 核心规则一：密钥指定了具体分组时，备用模型兜底**只能**在该分组内查找，
// 绝不允许跨分组，也不会因为其它分组有备用模型渠道而切过去。
func TestFallbackSearchGroups_SpecificGroupOnly(t *testing.T) {
	c := newGroupGatingContext(t, "vip", "default")

	groups := FallbackSearchGroups(c, "vip")
	require.Equal(t, []string{"vip"}, groups,
		"指定分组时只能搜索该分组，不能跨分组")
}

// TestFallbackSearchGroups_AutoUsesAutoChain
//
// 核心规则二：密钥是 auto 分组时，重走 auto 分组链，按顺序在各（有权限的）分组内查找。
func TestFallbackSearchGroups_AutoUsesAutoChain(t *testing.T) {
	c := newGroupGatingContext(t, "auto", "default")

	groups := FallbackSearchGroups(c, "default")
	require.NotEmpty(t, groups)
	// auto 链内的分组才可能被搜索；至少应包含当前分组
	require.Contains(t, groups, "default")
	// 去重
	seen := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		require.NotContains(t, seen, g, "分组列表不能重复")
		seen[g] = struct{}{}
	}
}

// TestFallbackSearchGroups_AutoCurrentGroupFirst
// auto 分组下应优先在当前命中分组内兜底，避免无谓地切分组。
func TestFallbackSearchGroups_AutoCurrentGroupFirst(t *testing.T) {
	c := newGroupGatingContext(t, "auto", "default")

	groups := FallbackSearchGroups(c, "default")
	require.NotEmpty(t, groups)
	require.Equal(t, "default", groups[0],
		"应优先在当前分组内兜底")
}

// TestFallbackSearchGroups_AutoRespectsUserPermission
//
// 与 auto 现有跨分组重试保持一致：只会在「用户有权限的分组」内查找。
//
// 构造依据（实测配置）：用户 default 的可用分组为 {default, vip}，auto 链为 [default]。
// 因此传入 currentGroup="vip"（用户有权限）时应被纳入搜索；
// 而 currentGroup="无权限分组" 时必须被剔除。
// 这条测试专门覆盖「权限过滤」分支，故意用有权限的分组做正向断言，
// 否则仅用瞎编的分组名会因为 auto 链里本来就没有它而误判通过。
func TestFallbackSearchGroups_AutoRespectsUserPermission(t *testing.T) {
	c := newGroupGatingContext(t, "auto", "default")

	// 用户有权限的分组：应出现在搜索结果首位
	allowed := FallbackSearchGroups(c, "vip")
	require.Contains(t, allowed, "vip",
		"用户有权限的分组应被纳入兜底搜索范围")

	// 用户无权限的分组：必须被剔除
	forbidden := FallbackSearchGroups(c, "完全无权限的分组")
	require.NotContains(t, forbidden, "完全无权限的分组",
		"不应把用户无权限的分组纳入兜底搜索范围")
}

// TestFallbackSearchGroups_NilSafe 容错
func TestFallbackSearchGroups_NilSafe(t *testing.T) {
	require.Nil(t, FallbackSearchGroups(nil, "default"))
}
