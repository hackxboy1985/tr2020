package poster

// ── 同步 AI 工具上游请求结构体 ──────────────────────────────────────────

// generateSyncRequest 对应 /openapi/v1/poster/generate（同步海报生成）
type generateSyncRequest struct {
	Query               string   `json:"query"`
	GenerateType        int      `json:"generateType,omitempty"`
	PosterType          int      `json:"posterType,omitempty"`
	PlatformType        string   `json:"platformType,omitempty"`
	LanguageType        string   `json:"languageType,omitempty"`
	DetailPictureNumber int      `json:"detailPictureNumber,omitempty"`
	ModelEdition        int      `json:"modelEdition,omitempty"`
	NeedText            *bool    `json:"needText,omitempty"`
	AspectRatio         string   `json:"aspectRatio,omitempty"`
	FileUrlList         []string `json:"fileUrlList,omitempty"`
	UserId              int64    `json:"userId,omitempty"`
}

type extensionRequest struct {
	ImgUrlList []string `json:"imgUrlList"`
	Ratio      string   `json:"ratio"`
}

type translateRequest struct {
	ImageUrl string `json:"imageUrl"`
	To       int    `json:"to"`
	From     string `json:"from,omitempty"`
}

type partialRedrawingRequest struct {
	SourceUrl       string `json:"sourceUrl"`
	TextPrompt      string `json:"textPrompt"`
	ReplaceImageUrl string `json:"replaceImageUrl,omitempty"`
}

type enlargeRequest struct {
	ImgUrls      string `json:"imgUrls"`
	ScalingRatio int    `json:"scalingRatio,omitempty"`
}

type mattingRequest struct {
	ImgUrls string `json:"imgUrls"`
}

type sceneReplaceRequest struct {
	SourceUrl       string `json:"sourceUrl"`
	ReplaceImageUrl string `json:"replaceImageUrl"`
	TextPrompt      string `json:"textPrompt"`
	ModelType       *int   `json:"modelType,omitempty"`
}

type productReplaceRequest struct {
	SourceUrl       string `json:"sourceUrl"`
	ReplaceImageUrl string `json:"replaceImageUrl"`
	TextPrompt      string `json:"textPrompt"`
	ModelType       *int   `json:"modelType,omitempty"`
}

type colorChangeRequest struct {
	SourceUrl  string `json:"sourceUrl"`
	TextPrompt string `json:"textPrompt"`
	ModelType  *int   `json:"modelType,omitempty"`
}

type enhanceRequest struct {
	ImgUrls         string `json:"imgUrls"`
	EnhanceStrength string `json:"enhanceStrength,omitempty"`
}

type assistedRequest struct {
	Query        string   `json:"query"`
	FileUrlList  []string `json:"fileUrlList,omitempty"`
	GenerateType string   `json:"generateType,omitempty"`
}

// ── 上游通用响应结构体 ──────────────────────────────────────────────────

// syncResponse covers sync endpoints that return a single string or comma-separated URLs in data
type syncResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data string `json:"data"`
}

// extensionSyncResponse covers /ai/extension which returns data as string array
type extensionSyncResponse struct {
	Code int      `json:"code"`
	Msg  string   `json:"msg"`
	Data []string `json:"data"`
}

// assistedResponse covers /ai/assisted which returns options array
type assistedResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Options []string `json:"options"`
	} `json:"data"`
}
