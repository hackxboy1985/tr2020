# 新增 KWJM 上游方案 - 技术方案

## 1. 需求概述

**目标：** 新增第二套上游转发方案，支持 KWJM 素材管理接口

**现状：** 
- ✅ 已有：Seedance Gateway 上游
- ❌ 新增：KWJM 上游

---

## 2. 接口差异对比

### 2.1 基本对比

| 项目 | Seedance Gateway | KWJM（新增） |
|------|-----------------|--------------|
| **基础URL** | `https://sd.dawnloadai.com:9444` | `https://kwjm.com` |
| **路径格式** | `/api/seedance/proxy/assets` | `/v3/open/CreateAsset` |
| **HTTP方法** | RESTful (GET/POST/DELETE) | POST（Action在路径中） |
| **请求体** | `{"name": "xxx"}` | `{"model": "kw-video-v2-fast", ...}` |
| **响应格式** | Gateway 自定义格式 | 类似火山官方格式 |

### 2.2 接口映射关系

| 功能 | Seedance Gateway | KWJM |
|------|-----------------|------|
| 创建资产分组 | `POST /api/seedance/proxy/assets/groups` | `POST /v3/open/CreateAssetGroup` |
| 列表资产分组 | `GET /api/seedance/proxy/assets/groups` | `POST /v3/open/ListAssetGroups` |
| 获取资产分组 | `GET /api/seedance/proxy/assets/groups/{id}` | `POST /v3/open/GetAssetGroup` |
| 更新资产分组 | `PUT /api/seedance/proxy/assets/groups/{id}` | `POST /v3/open/UpdateAssetGroup` |
| 删除资产分组 | `DELETE /api/seedance/proxy/assets/groups/{id}` | `POST /v3/open/DeleteAssetGroup` |
| 创建资产 | `POST /api/seedance/proxy/assets` | `POST /v3/open/CreateAsset` |
| 列表资产 | `GET /api/seedance/proxy/assets` | `POST /v3/open/ListAssets` |
| 获取资产 | `GET /api/seedance/proxy/assets/{id}` | `POST /v3/open/GetAsset` |
| 更新资产 | `PATCH /api/seedance/proxy/assets/{id}` | `POST /v3/open/UpdateAsset` |
| 删除资产 | `DELETE /api/seedance/proxy/assets/{id}` | `POST /v3/open/DeleteAsset` |

---

## 3. 配置设计

### 3.1 新增配置字段

**文件：** `dto/channel_settings.go`

```go
type ChannelOtherSettings struct {
    // ... 现有字段
    
    // Seedance Gateway 配置
    SeedanceAssetBaseUrl string `json:"seedance_asset_base_url,omitempty"`
    SeedanceRelayMode    bool   `json:"seedance_relay_mode,omitempty"`
    
    // KWJM 配置（新增）
    KwjmAssetBaseUrl     string `json:"kwjm_asset_base_url,omitempty"`
    KwjmAssetModel       string `json:"kwjm_asset_model,omitempty"` // 默认模型，如 sd-video-v2
    
    // 上游版本选择（新增）
    AssetUpstreamVersion string `json:"asset_upstream_version,omitempty"` // "gateway" | "kwjm"
}
```

### 3.2 配置示例

```json
{
  "seedance_asset_base_url": "https://sd.dawnloadai.com:9444",
  "kwjm_asset_base_url": "https://kwjm.com",
  "kwjm_asset_model": "sd-video-v2",
  "asset_upstream_version": "kwjm"
}
```

### 3.3 配置说明

| 字段 | 类型 | 必填 | 说明 | 默认值 |
|------|------|------|------|--------|
| `asset_upstream_version` | string | 否 | 上游版本选择 | `gateway` |
| `kwjm_asset_base_url` | string | 条件 | KWJM 基础URL | - |
| `kwjm_asset_model` | string | 否 | KWJM 默认模型 | `sd-video-v2` |

**配置优先级：**
1. 优先使用 `asset_upstream_version` 指定的版本
2. 未配置时默认使用 `gateway`
3. 根据版本读取对应的 base_url

---

## 4. 代码实现

### 4.1 修改 `service/seedance_proxy.go`

#### 4.1.1 扩展数据结构

```go
// AssetGatewayChannel 统一的资产网关渠道结构
// 重命名自 SeedanceGatewayChannel，支持多种上游
type AssetGatewayChannel struct {
    Channel         *model.Channel
    GatewayURL      string // 基础URL
    Key             string // API Key
    UpstreamVersion string // "gateway" | "kwjm"
    RelayMode       bool   // 仅 gateway 使用
    KwjmModel       string // 仅 kwjm 使用，默认模型名称
}
```

#### 4.1.2 修改渠道查询函数

```go
// GetAssetGatewayChannel 获取资产网关渠道
// 重命名自 GetSeedanceGatewayChannel，支持多种上游
func GetAssetGatewayChannel(userGroup string) (*AssetGatewayChannel, error) {
    channels, err := model.GetChannelsByType(0, 500, false, constant.ChannelTypeDoubaoVideo)
    if err != nil {
        return nil, fmt.Errorf("query channels failed: %w", err)
    }
    
    for _, ch := range channels {
        if !isGroupAllowed(ch, userGroup) || ch.Status != constant.ChannelStatusEnabled {
            continue
        }
        
        fullCh, err := model.GetChannelById(ch.Id, true)
        if err != nil || fullCh == nil {
            continue
        }
        
        settings := fullCh.GetOtherSettings()
        
        // 读取上游版本配置
        version := settings.AssetUpstreamVersion
        if version == "" {
            version = "gateway" // 默认使用 gateway
        }
        
        var gatewayURL string
        var kwjmModel string
        
        switch version {
        case "kwjm":
            gatewayURL = settings.KwjmAssetBaseUrl
            kwjmModel = settings.KwjmAssetModel
            if kwjmModel == "" {
                kwjmModel = "sd-video-v2" // 默认模型
            }
        case "gateway":
            gatewayURL = settings.SeedanceAssetBaseUrl
        default:
            // 未知版本，回退到 gateway
            gatewayURL = settings.SeedanceAssetBaseUrl
            version = "gateway"
        }
        
        if gatewayURL == "" {
            continue
        }
        
        key, _, apiErr := fullCh.GetNextEnabledKey()
        if apiErr != nil || key == "" {
            continue
        }
        
        return &AssetGatewayChannel{
            Channel:         fullCh,
            GatewayURL:      strings.TrimRight(gatewayURL, "/"),
            Key:             key,
            UpstreamVersion: version,
            RelayMode:       settings.SeedanceRelayMode,
            KwjmModel:       kwjmModel,
        }, nil
    }
    
    return nil, fmt.Errorf("no available asset gateway channel for group %s", userGroup)
}
```

#### 4.1.3 新增 KWJM 请求函数

```go
// KwjmProxyRequest 转发请求到 KWJM 上游
func KwjmProxyRequest(gc *AssetGatewayChannel, action string, query url.Values, body []byte) (int, []byte, error) {
    // 构造完整 URL
    targetURL := gc.GatewayURL + "/v3/open/" + action
    if len(query) > 0 {
        targetURL += "?" + query.Encode()
    }
    
    // 注入 model 字段到请求体
    if body != nil && len(body) > 0 {
        var reqBody map[string]interface{}
        if err := json.Unmarshal(body, &reqBody); err == nil {
            // 如果请求体没有 model 字段，注入默认 model
            if _, ok := reqBody["model"]; !ok {
                reqBody["model"] = gc.KwjmModel
            }
            body, _ = json.Marshal(reqBody)
        }
    }
    
    // 创建 HTTP 请求
    req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(body))
    if err != nil {
        return 0, nil, fmt.Errorf("create request failed: %w", err)
    }
    
    // 设置请求头
    req.Header.Set("Authorization", "Bearer "+gc.Key)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")
    
    common.SysLog(fmt.Sprintf("kwjm proxy: POST %s", targetURL))
    
    // 发送请求
    client, err := GetHttpClientWithProxy("")
    if err != nil {
        return 0, nil, fmt.Errorf("get http client failed: %w", err)
    }
    
    resp, err := client.Do(req)
    if err != nil {
        common.SysError(fmt.Sprintf("kwjm proxy do request failed: POST %s: %v", targetURL, err))
        return 0, nil, fmt.Errorf("do request failed: %w", err)
    }
    defer resp.Body.Close()
    
    // 读取响应
    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return resp.StatusCode, nil, fmt.Errorf("read response failed: %w", err)
    }
    
    // 记录错误响应
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        common.SysError(fmt.Sprintf("kwjm proxy upstream error: POST %s -> %d: %s", targetURL, resp.StatusCode, string(respBody)))
    }
    
    return resp.StatusCode, respBody, nil
}
```

#### 4.1.4 路径转 Action 映射函数

```go
// pathToKwjmAction 将 RESTful 路径和方法转换为 KWJM Action
func pathToKwjmAction(path, method string) string {
    // 资产分组相关
    if strings.Contains(path, "/assets/groups") {
        hasID := strings.Count(path, "/") > 5 // 路径包含 ID
        
        switch method {
        case http.MethodPost:
            return "CreateAssetGroup"
        case http.MethodGet:
            if hasID {
                return "GetAssetGroup"
            }
            return "ListAssetGroups"
        case http.MethodPut, http.MethodPatch:
            return "UpdateAssetGroup"
        case http.MethodDelete:
            return "DeleteAssetGroup"
        }
    }
    
    // 资产相关
    if strings.Contains(path, "/assets") && !strings.Contains(path, "/assets/groups") {
        hasID := strings.Count(path, "/") > 4 // 路径包含 ID
        
        switch method {
        case http.MethodPost:
            return "CreateAsset"
        case http.MethodGet:
            if hasID {
                return "GetAsset"
            }
            return "ListAssets"
        case http.MethodPut, http.MethodPatch:
            return "UpdateAsset"
        case http.MethodDelete:
            return "DeleteAsset"
        }
    }
    
    // 真人认证相关
    if strings.Contains(path, "/face-verifications") {
        hasID := strings.Count(path, "/") > 3
        
        switch method {
        case http.MethodPost:
            return "CreateVisualValidateSession"
        case http.MethodGet:
            if hasID {
                return "GetVisualValidateResult"
            }
        }
    }
    
    return ""
}
```

#### 4.1.5 统一请求分发函数

```go
// AssetProxyRequest 统一的资产代理请求函数
// 根据上游版本自动选择转发方式
func AssetProxyRequest(gc *AssetGatewayChannel, method, path string, query url.Values, body []byte) (int, []byte, error) {
    switch gc.UpstreamVersion {
    case "kwjm":
        // KWJM 上游：转换路径为 Action
        action := pathToKwjmAction(path, method)
        if action == "" {
            return 0, nil, fmt.Errorf("unsupported path for kwjm: %s %s", method, path)
        }
        return KwjmProxyRequest(gc, action, query, body)
        
    case "gateway":
        // Gateway 上游：保持原有逻辑
        return SeedanceProxyRequest(gc, method, path, query, body)
        
    default:
        return 0, nil, fmt.Errorf("unsupported upstream version: %s", gc.UpstreamVersion)
    }
}
```

#### 4.1.6 重命名原有函数

```go
// SeedanceProxyRequest 保持原有 Gateway 逻辑不变
// 只需将函数签名中的 SeedanceGatewayChannel 改为 AssetGatewayChannel
func SeedanceProxyRequest(gc *AssetGatewayChannel, method, upstreamPath string, query url.Values, body []byte) (int, []byte, error) {
    // 原有逻辑保持不变
    targetURL := gc.GatewayURL + upstreamPath
    // ... 其余代码不变
}
```

---

### 4.2 修改 `controller/seedance.go`

#### 4.2.1 更新类型引用

```go
// 所有 *service.SeedanceGatewayChannel 改为 *service.AssetGatewayChannel
func SeedanceCreateAssetGroup(c *gin.Context) {
    // ...
    gw, err := service.GetAssetGatewayChannel(userGroup)
    // ...
}
```

#### 4.2.2 更新 proxyAndPassthrough 函数

```go
func proxyAndPassthrough(c *gin.Context, gw *service.AssetGatewayChannel, method, path string, query url.Values, body []byte, onSuccess func(statusCode int, body []byte)) {
    // 使用统一的代理函数
    statusCode, respBody, err := service.AssetProxyRequest(gw, method, path, query, body)
    if err != nil {
        c.JSON(http.StatusBadGateway, gin.H{
            "code":    "gateway_error",
            "message": err.Error(),
        })
        return
    }
    
    if statusCode >= 400 {
        c.Data(statusCode, "application/json", respBody)
        return
    }
    
    if onSuccess != nil {
        onSuccess(statusCode, respBody)
    } else {
        c.Data(statusCode, "application/json", respBody)
    }
}
```

#### 4.2.3 批量替换函数调用

```bash
# 全局替换
GetSeedanceGatewayChannel → GetAssetGatewayChannel
SeedanceGatewayChannel → AssetGatewayChannel
SeedanceProxyRequest → AssetProxyRequest (仅在需要的地方)
```

---

## 5. 响应格式处理

### 5.1 响应格式对比

**Seedance Gateway 响应：**
```json
{
  "id": "group-xxx",
  "name": "测试分组",
  "created_at": 1234567890
}
```

**KWJM 响应（类似火山官方）：**
```json
{
  "Id": "group-xxx",
  "Name": "测试分组",
  "CreateTime": 1234567890
}
```

### 5.2 响应处理策略

**保持现有逻辑：**
- KWJM 响应格式已经接近火山官方格式
- 现有的 `doubao_official_format` 转换逻辑可以复用
- 可能需要微调部分字段映射

**字段映射：**
```go
// Gateway → 标准格式
id → Id
name → Name
created_at → CreateTime
updated_at → UpdateTime

// KWJM → 标准格式
// KWJM 已经是标准格式，无需映射
```

---

## 6. 测试方案

### 6.1 单元测试

**文件：** `service/seedance_proxy_test.go`

```go
package service

import "testing"

func TestPathToKwjmAction(t *testing.T) {
    tests := []struct {
        path   string
        method string
        want   string
    }{
        // 资产分组
        {"/api/seedance/proxy/assets/groups", "POST", "CreateAssetGroup"},
        {"/api/seedance/proxy/assets/groups", "GET", "ListAssetGroups"},
        {"/api/seedance/proxy/assets/groups/group-123", "GET", "GetAssetGroup"},
        {"/api/seedance/proxy/assets/groups/group-123", "PUT", "UpdateAssetGroup"},
        {"/api/seedance/proxy/assets/groups/group-123", "DELETE", "DeleteAssetGroup"},
        
        // 资产
        {"/api/seedance/proxy/assets", "POST", "CreateAsset"},
        {"/api/seedance/proxy/assets", "GET", "ListAssets"},
        {"/api/seedance/proxy/assets/asset-123", "GET", "GetAsset"},
        {"/api/seedance/proxy/assets/asset-123", "PATCH", "UpdateAsset"},
        {"/api/seedance/proxy/assets/asset-123", "DELETE", "DeleteAsset"},
        
        // 真人认证
        {"/api/seedance/face-verifications", "POST", "CreateVisualValidateSession"},
        {"/api/seedance/face-verifications/session-123", "GET", "GetVisualValidateResult"},
    }
    
    for _, tt := range tests {
        t.Run(tt.method+" "+tt.path, func(t *testing.T) {
            got := pathToKwjmAction(tt.path, tt.method)
            if got != tt.want {
                t.Errorf("pathToKwjmAction(%q, %q) = %q, want %q", tt.path, tt.method, got, tt.want)
            }
        })
    }
}

func TestPathToKwjmAction_Unsupported(t *testing.T) {
    got := pathToKwjmAction("/api/unknown", "GET")
    if got != "" {
        t.Errorf("pathToKwjmAction for unknown path should return empty string, got %q", got)
    }
}
```

### 6.2 集成测试清单

**测试环境配置：**
```json
{
  "asset_upstream_version": "kwjm",
  "kwjm_asset_base_url": "https://kwjm-test.com",
  "kwjm_asset_model": "sd-video-v2"
}
```

**测试用例：**

| 序号 | 功能 | 接口 | 预期结果 |
|------|------|------|----------|
| 1 | 创建资产分组 | `POST /api/seedance/asset-groups` | 返回分组ID |
| 2 | 列表资产分组 | `GET /api/seedance/asset-groups` | 返回分组列表 |
| 3 | 获取资产分组 | `GET /api/seedance/asset-groups/:id` | 返回分组详情 |
| 4 | 更新资产分组 | `PUT /api/seedance/asset-groups/:id` | 更新成功 |
| 5 | 删除资产分组 | `DELETE /api/seedance/asset-groups/:id` | 删除成功 |
| 6 | 创建资产 | `POST /api/seedance/assets` | 返回资产ID |
| 7 | 列表资产 | `GET /api/seedance/assets` | 返回资产列表 |
| 8 | 获取资产 | `GET /api/seedance/assets/:id` | 返回资产详情 |
| 9 | 更新资产 | `PATCH /api/seedance/assets/:id` | 更新成功 |
| 10 | 删除资产 | `DELETE /api/seedance/assets/:id` | 删除成功 |
| 11 | 火山格式创建 | `POST /api/seedance/assets/v2?Action=CreateAsset` | 返回火山格式 |
| 12 | 火山格式列表 | `POST /api/seedance/assets/v2?Action=ListAssets` | 返回火山格式 |

### 6.3 回归测试

**测试 Gateway 上游是否受影响：**

```json
{
  "asset_upstream_version": "gateway",
  "seedance_asset_base_url": "https://sd.dawnloadai.com:9444"
}
```

重复上述所有测试用例，确保 Gateway 上游功能正常。

---

## 7. 兼容性保障

### 7.1 向后兼容策略

**默认行为：**
- `asset_upstream_version` 未配置时，默认使用 `gateway`
- 现有配置无需修改，行为保持不变

**配置迁移：**
```json
// 旧配置（继续有效）
{
  "seedance_asset_base_url": "https://sd.dawnloadai.com:9444"
}

// 新配置（显式指定）
{
  "asset_upstream_version": "gateway",
  "seedance_asset_base_url": "https://sd.dawnloadai.com:9444"
}
```

### 7.2 灰度切换方案

**阶段1：部署新代码**
- 所有渠道保持 `gateway` 配置
- 验证功能正常

**阶段2：灰度测试**
- 选择1-2个测试渠道切换到 `kwjm`
- 监控错误日志和用户反馈

**阶段3：分批切换**
- 按用户分组逐步切换
- 每批次间隔 1-2 天

**阶段4：全量切换**
- 所有渠道切换到 `kwjm`
- 保留 Gateway 配置作为回退方案

**回退方案：**
```json
// 快速回退到 Gateway
{
  "asset_upstream_version": "gateway"
}
```

---

## 8. 监控和日志

### 8.1 关键日志

**成功日志：**
```
[SYS] kwjm proxy: POST https://kwjm.com/v3/open/CreateAsset
```

**错误日志：**
```
[ERR] kwjm proxy upstream error: POST https://kwjm.com/v3/open/CreateAsset -> 400: {...}
[ERR] kwjm proxy do request failed: POST https://kwjm.com/v3/open/CreateAsset: connection timeout
```

### 8.2 监控指标

**需要监控的指标：**
- KWJM 上游请求成功率
- KWJM 上游响应时间
- KWJM 上游错误类型分布
- Gateway vs KWJM 性能对比

**告警规则：**
- 成功率 < 95%：告警
- P99 响应时间 > 3s：告警
- 5xx 错误率 > 1%：告警

---

## 9. 部署步骤

### 9.1 代码开发

1. **修改配置定义** (`dto/channel_settings.go`)
2. **修改核心逻辑** (`service/seedance_proxy.go`)
3. **更新控制器** (`controller/seedance.go`)
4. **添加单元测试** (`service/seedance_proxy_test.go`)

### 9.2 本地测试

```bash
# 1. 编译
go build -o new-api

# 2. 配置测试环境
# 在后台修改渠道配置

# 3. 运行测试
go test ./service -v -run TestPathToKwjmAction

# 4. 手动测试接口
curl -X POST http://localhost:3000/api/seedance/assets \
  -H "Authorization: Bearer token" \
  -d '{"url": "https://example.com/image.jpg"}'
```

### 9.3 灰度发布

```bash
# 1. 部署到测试环境
./deploy.sh test

# 2. 选择测试渠道，修改配置
UPDATE channels 
SET other_settings = JSON_SET(
    other_settings, 
    '$.asset_upstream_version', 'kwjm',
    '$.kwjm_asset_base_url', 'https://kwjm.com',
    '$.kwjm_asset_model', 'kw-video-v2-fast'
)
WHERE id = 测试渠道ID;

# 3. 监控日志
tail -f logs/oneapi-*.log | grep -E "kwjm|asset"

# 4. 验证功能
# 运行集成测试清单中的所有用例
```

### 9.4 全量发布

```bash
# 1. 部署到生产环境
./deploy.sh production

# 2. 分批更新渠道配置
# 第一批：10%
# 第二批：30%
# 第三批：60%
# 第四批：100%

# 3. 持续监控
# 观察错误日志、成功率、响应时间
```

---

## 10. 风险评估与缓解

| 风险 | 等级 | 影响 | 概率 | 缓解措施 |
|------|------|------|------|---------|
| KWJM 接口不稳定 | 高 | 请求失败 | 中 | 保留 Gateway 配置，1分钟内可回退 |
| 响应格式不兼容 | 中 | 解析失败 | 低 | 充分测试，添加格式兼容逻辑 |
| 配置错误 | 中 | 服务不可用 | 低 | 添加配置校验，完善错误提示 |
| 性能下降 | 低 | 响应变慢 | 低 | 性能测试，监控响应时间 |
| 路径映射错误 | 中 | 请求失败 | 中 | 单元测试覆盖所有路径 |

**风险处置预案：**
1. **立即回退：** 修改配置 `asset_upstream_version` 为 `gateway`
2. **降级服务：** 暂停受影响用户的资产管理功能
3. **紧急修复：** 热修复代码，快速发布
4. **通知用户：** 系统公告说明情况和恢复时间

---

## 11. 时间估算

| 任务 | 预计工时 | 备注 |
|------|---------|------|
| 配置定义修改 | 0.5h | 简单 |
| 核心逻辑开发 | 4h | 中等复杂度 |
| 控制器更新 | 1h | 批量替换 |
| 单元测试编写 | 2h | 覆盖主要场景 |
| 本地集成测试 | 2h | 手动测试所有接口 |
| 代码审查 | 1h | - |
| 文档编写 | 1h | 本文档 |
| 灰度测试 | 2h | 监控和验证 |
| **总计** | **13.5h** | 约 2 个工作日 |

---

## 12. 文档更新

### 12.1 需要更新的文档

- [ ] API 文档：说明新增配置项
- [ ] 运维手册：添加灰度切换步骤
- [ ] 故障排查：添加 KWJM 相关错误处理

### 12.2 用户通知

**标题：** 资产管理接口升级通知

**内容：**
> 尊敬的用户，
> 
> 我们将于 [日期] 对资产管理接口进行升级，新增 KWJM 上游支持。
> 
> **升级内容：**
> - 新增 KWJM 上游配置选项
> - 保持现有 Gateway 上游完全兼容
> - 用户无需修改现有代码
> 
> **影响范围：**
> - 资产创建、查询、更新、删除接口
> - 资产分组管理接口
> 
> **注意事项：**
> - 升级过程中服务不中断
> - 如遇问题请及时联系技术支持
> 
> 感谢您的支持！

---

## 13. 附录

### 13.1 KWJM 接口文档参考

详见 `docs/kwjm/素材.md`

### 13.2 配置字段完整列表

```go
type ChannelOtherSettings struct {
    // Doubao 视频生成相关
    DoubaoVideoGeneratePath string `json:"doubao_video_generate_path,omitempty"`
    DoubaoVideoFetchPath    string `json:"doubao_video_fetch_path,omitempty"`
    
    // Seedance Gateway 资产管理
    SeedanceAssetBaseUrl    string `json:"seedance_asset_base_url,omitempty"`
    SeedanceRelayMode       bool   `json:"seedance_relay_mode,omitempty"`
    
    // KWJM 资产管理（新增）
    KwjmAssetBaseUrl        string `json:"kwjm_asset_base_url,omitempty"`
    KwjmAssetModel          string `json:"kwjm_asset_model,omitempty"`
    
    // 上游版本选择（新增）
    AssetUpstreamVersion    string `json:"asset_upstream_version,omitempty"` // "gateway" | "kwjm"
}
```

### 13.3 错误码说明

| 错误码 | 说明 | 处理方式 |
|--------|------|---------|
| `gateway_error` | 网关错误 | 检查网络连接和配置 |
| `unsupported_upstream` | 不支持的上游版本 | 检查 `asset_upstream_version` 配置 |
| `unsupported_path` | 不支持的路径 | 检查接口路径是否正确 |
| `auth_failed` | 认证失败 | 检查 API Key 是否正确 |

---

## 🎯 总结

**新增 KWJM 上游方案核心要点：**

1. **配置驱动**：通过 `asset_upstream_version` 灵活切换上游
2. **向后兼容**：默认 `gateway`，现有配置无需修改
3. **统一接口**：下游接口保持不变，上游自动适配
4. **易于扩展**：未来可以轻松添加第三套上游

**下一步行动：**
- [ ] 开始编码实现
- [ ] 编写单元测试
- [ ] 本地集成测试
- [ ] 提交代码审查
- [ ] 灰度发布

---

**文档版本：** v1.0  
**创建日期：** 2026-09-04  
**最后更新：** 2026-09-04  
**作者：** Claude Code
