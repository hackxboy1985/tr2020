package controller

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

// GetAllGroups 获取所有分组列表（用于配置页面）
func GetAllGroups(c *gin.Context) {
	var groups []string
	// 查询所有分组（不过滤 enabled，管理员配置时应该能看到所有分组）
	err := model.DB.Model(&model.Ability{}).
		Distinct("group").
		Pluck("group", &groups).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	// 读取当前配置
	selectedGroups := model.GetGroupMonitorVisibleGroups()

	// 构建返回数据
	result := []gin.H{}
	for _, group := range groups {
		selected := true // 默认全选
		if selectedGroups != nil {
			selected = lo.Contains(selectedGroups, group)
		}

		result = append(result, gin.H{
			"name":     group,
			"selected": selected,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// SaveGroupMonitorConfig 保存分组监控配置
func SaveGroupMonitorConfig(c *gin.Context) {
	var req struct {
		VisibleGroups []string `json:"visible_groups"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	err := model.SetGroupMonitorVisibleGroups(req.VisibleGroups)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "配置已保存"})
}

// GroupMonitorResult 分组监控结果
type GroupMonitorResult struct {
	Group                          string                   `json:"group"`
	Status                         string                   `json:"status"` // up / degraded / down
	Uptime24h                      float64                  `json:"uptime_24h"`
	AvgLatency                     int                      `json:"avg_latency"`
	LastTestedAt                   int64                    `json:"last_tested_at"`
	TopChannels                    []ChannelInfo            `json:"top_channels"`
	DisabledHigherPriorityChannels []DisabledChannelInfo    `json:"disabled_higher_priority_channels"`
	Heartbeats                     []HeartbeatRecord        `json:"heartbeats"`
}

type ChannelInfo struct {
	ID           *int    `json:"id,omitempty"`
	Name         *string `json:"name,omitempty"`
	DisplayName  string  `json:"display_name,omitempty"`
	Priority     int64   `json:"priority"`
	ResponseTime int     `json:"response_time"`
	Status       int     `json:"status"`
}

type DisabledChannelInfo struct {
	ID          *int    `json:"id,omitempty"`
	Name        *string `json:"name,omitempty"`
	DisplayName string  `json:"display_name,omitempty"`
	Priority    int64   `json:"priority"`
	Status      int     `json:"status"`
}

type HeartbeatRecord struct {
	TestedAt int64              `json:"tested_at"`
	Color    string             `json:"color"` // green / yellow / orange / red
	TestModel string            `json:"test_model,omitempty"`
	Results  map[int]TestResult `json:"results"`
}

type TestResult struct {
	Success      bool `json:"success"`
	ResponseTime int  `json:"response_time"`
}

// GetGroupMonitorStatus 获取分组监控状态
func GetGroupMonitorStatus(c *gin.Context) {
	userRole := c.GetInt("role")
	isAdmin := userRole >= common.RoleAdminUser

	// 读取要显示的分组配置
	visibleGroups := model.GetGroupMonitorVisibleGroups()

	// 如果未配置，则显示所有分组
	if visibleGroups == nil {
		model.DB.Model(&model.Ability{}).
			Distinct("group").
			Where("enabled = ?", true).
			Pluck("group", &visibleGroups)
	}

	// 普通用户：过滤出该用户有权访问的分组
	if !isAdmin {
		userId := c.GetInt("id")
		userGroup, _ := model.GetUserGroup(userId, false)
		userUsableGroups := service.GetUserUsableGroups(userGroup)

		filtered := []string{}
		for _, g := range visibleGroups {
			if _, ok := userUsableGroups[g]; ok {
				filtered = append(filtered, g)
			}
		}
		visibleGroups = filtered
	}

	// 对每个可见分组，计算监控数据
	results := []GroupMonitorResult{}
	for _, groupName := range visibleGroups {
		groupStatus := calculateGroupMonitorStatus(groupName, isAdmin)
		if groupStatus != nil {
			results = append(results, *groupStatus)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}

// calculateGroupMonitorStatus 计算分组监控状态
func calculateGroupMonitorStatus(groupName string, isAdmin bool) *GroupMonitorResult {
	// 1. 查询该分组的所有渠道，按优先级降序
	var abilities []model.Ability
	err := model.DB.Where("`group` = ?", groupName).
		Order("priority DESC, channel_id ASC").
		Find(&abilities).Error

	if err != nil || len(abilities) == 0 {
		return nil
	}

	// 2. 取前 3 个优先级最高的渠道
	top3Count := 3
	if len(abilities) < 3 {
		top3Count = len(abilities)
	}
	top3Abilities := abilities[:top3Count]

	// 3. 查询这些渠道的详细信息
	channelIds := lo.Map(top3Abilities, func(a model.Ability, _ int) int {
		return a.ChannelId
	})

	var channels []model.Channel
	model.DB.Where("id IN ?", channelIds).Find(&channels)
	channelMap := lo.SliceToMap(channels, func(c model.Channel) (int, model.Channel) {
		return c.Id, c
	})

	// 4. 生成渠道显示名称（编号）
	displayNames := generateChannelDisplayNames(abilities)

	// 5. 构建 top_channels 信息
	topChannels := []ChannelInfo{}
	for _, ability := range top3Abilities {
		channel, exists := channelMap[ability.ChannelId]
		if !exists {
			continue
		}

		info := ChannelInfo{
			DisplayName:  displayNames[ability.ChannelId],
			Priority:     *ability.Priority,
			ResponseTime: channel.ResponseTime,
			Status:       channel.Status,
		}

		if isAdmin {
			info.ID = &channel.Id
			info.Name = &channel.Name
		}

		topChannels = append(topChannels, info)
	}

	// 6. 查询测试历史（最近 60 条）
	histories, err := model.GetMultiChannelTestHistories(channelIds, 60)
	if err != nil {
		histories = make(map[int][]model.ChannelTestHistory)
	}

	// 7. 计算心跳格颜色
	heartbeats := calculateHeartbeats(top3Abilities, histories)

	// 8. 计算可用率和平均延迟
	uptime, avgLatency := calculateUptimeAndLatency(heartbeats)

	// 9. 判断分组状态和降级渠道
	status, disabledChannels := calculateGroupStatus(abilities, channelMap, displayNames, isAdmin)

	// 10. 最后测试时间
	lastTestedAt := int64(0)
	for _, h := range heartbeats {
		if h.TestedAt > lastTestedAt {
			lastTestedAt = h.TestedAt
		}
	}

	return &GroupMonitorResult{
		Group:                          groupName,
		Status:                         status,
		Uptime24h:                      uptime,
		AvgLatency:                     avgLatency,
		LastTestedAt:                   lastTestedAt,
		TopChannels:                    topChannels,
		DisabledHigherPriorityChannels: disabledChannels,
		Heartbeats:                     heartbeats,
	}
}

// generateChannelDisplayNames 生成渠道显示名称（渠道 #1, #2, #3...）
func generateChannelDisplayNames(abilities []model.Ability) map[int]string {
	displayNames := make(map[int]string)
	for idx, ability := range abilities {
		displayNames[ability.ChannelId] = fmt.Sprintf("渠道 #%d", idx+1)
	}
	return displayNames
}

// calculateHeartbeats 计算心跳格
func calculateHeartbeats(top3Abilities []model.Ability, histories map[int][]model.ChannelTestHistory) []HeartbeatRecord {
	// 按时间点分组，同时记录 test_model
	timeMap := make(map[int64]map[int]TestResult)
	timeModelMap := make(map[int64]string) // 每个时间点的测试模型

	for _, ability := range top3Abilities {
		channelHistories := histories[ability.ChannelId]
		for _, h := range channelHistories {
			if _, exists := timeMap[h.TestedAt]; !exists {
				timeMap[h.TestedAt] = make(map[int]TestResult)
			}
			timeMap[h.TestedAt][ability.ChannelId] = TestResult{
				Success:      h.Success,
				ResponseTime: h.ResponseTime,
			}
			// 取第一个有值的 test_model
			if h.TestModel != "" && timeModelMap[h.TestedAt] == "" {
				timeModelMap[h.TestedAt] = h.TestModel
			}
		}
	}

	// 转换为数组并排序
	times := lo.Keys(timeMap)
	sort.Slice(times, func(i, j int) bool {
		return times[i] > times[j] // 降序
	})

	// 限制 60 条
	if len(times) > 60 {
		times = times[:60]
	}

	heartbeats := []HeartbeatRecord{}
	for _, t := range times {
		results := timeMap[t]
		color := calculateHeartbeatColor(top3Abilities, results)

		heartbeats = append(heartbeats, HeartbeatRecord{
			TestedAt:  t,
			Color:     color,
			TestModel: timeModelMap[t],
			Results:   results,
		})
	}

	return heartbeats
}

// calculateHeartbeatColor 计算心跳格颜色
func calculateHeartbeatColor(top3Abilities []model.Ability, results map[int]TestResult) string {
	// 按优先级顺序判断
	for _, ability := range top3Abilities {
		if result, exists := results[ability.ChannelId]; exists && result.Success {
			// 找到第一个成功的渠道
			for i, a := range top3Abilities {
				if a.ChannelId == ability.ChannelId {
					switch i {
					case 0:
						return "green" // 最高优先级成功
					case 1:
						return "yellow" // 中优先级成功（高优先级失败）
					case 2:
						return "orange" // 低优先级成功（前2个失败）
					}
				}
			}
		}
	}

	// 所有渠道都失败
	return "red"
}

// calculateUptimeAndLatency 计算可用率和平均延迟
func calculateUptimeAndLatency(heartbeats []HeartbeatRecord) (float64, int) {
	if len(heartbeats) == 0 {
		return 0, 0
	}

	upCount := 0
	totalLatency := 0
	latencyCount := 0

	for _, hb := range heartbeats {
		if hb.Color != "red" {
			upCount++
		}

		// 计算平均延迟（只算成功的）
		for _, result := range hb.Results {
			if result.Success && result.ResponseTime > 0 {
				totalLatency += result.ResponseTime
				latencyCount++
			}
		}
	}

	uptime := float64(upCount) / float64(len(heartbeats))
	avgLatency := 0
	if latencyCount > 0 {
		avgLatency = totalLatency / latencyCount
	}

	return uptime, avgLatency
}

// calculateGroupStatus 计算分组状态和降级渠道
func calculateGroupStatus(abilities []model.Ability, channelMap map[int]model.Channel, displayNames map[int]string, isAdmin bool) (string, []DisabledChannelInfo) {
	// 找出第一个 enabled=true 的渠道
	firstEnabledIdx := -1
	for i, ability := range abilities {
		channel := channelMap[ability.ChannelId]
		if channel.Status == 1 { // enabled
			firstEnabledIdx = i
			break
		}
	}

	// 所有渠道都禁用
	if firstEnabledIdx == -1 {
		return "down", []DisabledChannelInfo{}
	}

	// 检查是否有更高优先级的渠道被禁用
	disabledChannels := []DisabledChannelInfo{}
	for i := 0; i < firstEnabledIdx; i++ {
		ability := abilities[i]
		channel := channelMap[ability.ChannelId]

		info := DisabledChannelInfo{
			DisplayName: displayNames[ability.ChannelId],
			Priority:    *ability.Priority,
			Status:      channel.Status,
		}

		if isAdmin {
			info.ID = &channel.Id
			info.Name = &channel.Name
		}

		disabledChannels = append(disabledChannels, info)
	}

	// 有更高优先级渠道被禁用 = 降级
	if len(disabledChannels) > 0 {
		return "degraded", disabledChannels
	}

	// 正常
	return "up", []DisabledChannelInfo{}
}
