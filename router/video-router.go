package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetVideoRouter(router *gin.Engine) {
	// Video proxy: accepts either session auth (dashboard) or token auth (API clients)
	videoProxyRouter := router.Group("/v1")
	videoProxyRouter.Use(middleware.RouteTag("relay"))
	videoProxyRouter.Use(middleware.TokenOrUserAuth())
	{
		videoProxyRouter.GET("/videos/:task_id/content", controller.VideoProxy)
	}

	videoV1Router := router.Group("/v1")
	videoV1Router.Use(middleware.RouteTag("relay"))
	videoV1Router.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		videoV1Router.POST("/video/generations", controller.RelayTask)
		videoV1Router.GET("/video/generations/:task_id", controller.RelayTaskFetch)
		videoV1Router.POST("/videos/:video_id/remix", controller.RelayTask)
	}
	// openai compatible API video routes
	{
		videoV1Router.POST("/videos", controller.RelayTask)
		videoV1Router.GET("/videos/:task_id", controller.RelayTaskFetch)
	}

	klingV1Router := router.Group("/kling/v1")
	klingV1Router.Use(middleware.RouteTag("relay"))
	klingV1Router.Use(middleware.KlingRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		klingV1Router.POST("/videos/text2video", controller.RelayTask)
		klingV1Router.POST("/videos/image2video", controller.RelayTask)
		klingV1Router.GET("/videos/text2video/:task_id", controller.RelayTaskFetch)
		klingV1Router.GET("/videos/image2video/:task_id", controller.RelayTaskFetch)
	}

	// Doubao official API routes
	doubaoOfficialGroup := router.Group("/api/v3/contents/generations")
	doubaoOfficialGroup.Use(middleware.RouteTag("relay"))
	doubaoOfficialGroup.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		doubaoOfficialGroup.POST("/tasks", controller.RelayTask)
		doubaoOfficialGroup.GET("/tasks/:task_id", controller.RelayTaskFetch)
	}

	// Jimeng official API routes
	jimengOfficialGroup := router.Group("jimeng")
	jimengOfficialGroup.Use(middleware.RouteTag("relay"))
	jimengOfficialGroup.Use(middleware.JimengRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		jimengOfficialGroup.POST("/", controller.RelayTask)
	}

	// 视频生成项目管理（用户 + 管理员共用路由，内部按 role 分流）
	videoGenRouter := router.Group("/api/video-generation")
	videoGenRouter.Use(middleware.RouteTag("api"))
	videoGenRouter.Use(middleware.TokenAuth())
	{
		videoGenRouter.POST("/create", controller.CreateVideoProject)
		videoGenRouter.GET("/projects", controller.ListVideoProjects)
		videoGenRouter.GET("/projects/:id", controller.GetVideoProject)
		videoGenRouter.DELETE("/projects/:id", controller.DeleteVideoProject)
		videoGenRouter.PUT("/admin/projects/:id/status", controller.UpdateVideoProjectStatus)
		// 注：DELETE /projects/:id 已支持管理员删任意项目（role 分流），无需重复路由
	}

	// 渠道管理（仅管理员）
	videoChannelRouter := router.Group("/api/video-generation/channels")
	videoChannelRouter.Use(middleware.RouteTag("api"))
	videoChannelRouter.Use(middleware.UserAuth(), middleware.AdminAuth())
	{
		videoChannelRouter.GET("", controller.ListVideoChannels)
		videoChannelRouter.POST("", controller.CreateVideoChannel)
		videoChannelRouter.PUT("/:id", controller.UpdateVideoChannel)
		videoChannelRouter.DELETE("/:id", controller.DeleteVideoChannel)
		videoChannelRouter.PUT("/:id/status", controller.UpdateVideoChannelStatus)
	}

	// Webhook 回调（无需认证，通过签名验证）
	router.POST("/api/video-generation/webhook/:channel_id", controller.HandleWebhook)
}
