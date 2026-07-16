package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func SetSeedanceRouter(router *gin.Engine) {
	// 用户侧：TokenAuth 验证，透传到上游 Gateway
	userGroup := router.Group("/api/seedance")
	userGroup.Use(middleware.RouteTag("api"))
	userGroup.Use(middleware.TokenOrUserAuth())
	{
		// 素材组
		userGroup.POST("/asset-groups", controller.SeedanceCreateAssetGroup)
		userGroup.GET("/asset-groups", controller.SeedanceListAssetGroups)
		userGroup.GET("/asset-groups/:id", controller.SeedanceGetAssetGroup)
		userGroup.PUT("/asset-groups/:id", controller.SeedancePutAssetGroup)
		userGroup.PATCH("/asset-groups/:id", controller.SeedancePatchAssetGroup)
		userGroup.DELETE("/asset-groups/:id", controller.SeedanceDeleteAssetGroup)

		// 素材
		userGroup.POST("/assets", controller.SeedanceCreateAsset)
		userGroup.GET("/assets", controller.SeedanceListAssets)
		userGroup.GET("/assets/:id", controller.SeedanceGetAsset)
		userGroup.PUT("/assets/:id", controller.SeedancePutAsset)
		userGroup.PATCH("/assets/:id", controller.SeedancePatchAsset)
		userGroup.DELETE("/assets/:id", controller.SeedanceDeleteAsset)

		// 人脸认证
		userGroup.POST("/face-verifications", controller.SeedanceCreateFaceVerification)
		userGroup.GET("/face-verifications", controller.SeedanceListFaceVerifications)
		userGroup.GET("/face-verifications/:id", controller.SeedanceGetFaceVerification)
	}

	// 管理员侧：查看本地表数据
	adminGroup := router.Group("/api/admin/seedance")
	adminGroup.Use(middleware.RouteTag("api"))
	adminGroup.Use(middleware.UserAuth(), middleware.AdminAuth())
	{
		adminGroup.GET("/asset-groups", controller.SeedanceAdminListAssetGroups)
		adminGroup.GET("/assets", controller.SeedanceAdminListAssets)
		adminGroup.GET("/face-verifications", controller.SeedanceAdminListFaceVerifications)
	}
}
