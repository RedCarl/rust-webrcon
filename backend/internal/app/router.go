package app

import (
	"strings"

	"github.com/gin-gonic/gin"
)

type apiErrorBody struct {
	Message string `json:"message"`
}

func (a *App) Router() *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	router.Static("/assets", "./dist/assets")
	router.StaticFile("/", "./dist/index.html")
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, apiErrorBody{Message: "未找到"})
			return
		}
		c.File("./dist/index.html")
	})

	api := router.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", a.ginRegister)
			auth.POST("/login", a.ginLogin)
		}

		api.GET("/servers/:id/last-wipe", a.ginGetServerLastWipe)
		api.POST("/servers/:id/command", a.ginServerCommand)

		protected := api.Group("/")
		protected.Use(a.ginAuthRequired())
		{
			protected.GET("/me", a.ginMe)
			protected.GET("/servers", a.ginListServers)
			protected.POST("/servers", a.ginCreateServer)
			protected.PUT("/servers/:id", a.ginUpdateServer)
			protected.DELETE("/servers/:id", a.ginDeleteServer)
			protected.POST("/servers/:id/wipes", a.ginExecuteServerWipe)
			protected.POST("/servers/:id/actions/:action", a.ginServerAction)

			admin := protected.Group("/admin")
			admin.Use(a.ginAdminRequired())
			{
				admin.GET("/settings/wipe", a.ginAdminGetWipeSettings)
				admin.PUT("/settings/wipe", a.ginAdminUpdateWipeSettings)

				admin.GET("/users", a.ginAdminListUsers)
				admin.POST("/users", a.ginAdminCreateUser)
				admin.PUT("/users/:id/password", a.ginAdminUpdateUserPassword)
				admin.PUT("/users/:id/servers", a.ginAdminAssignUserServers)

				admin.GET("/servers", a.ginAdminListServers)
				admin.POST("/servers", a.ginAdminCreateServer)
				admin.PUT("/servers/:id", a.ginAdminUpdateServer)
				admin.DELETE("/servers/:id", a.ginAdminDeleteServer)

				admin.GET("/player-data/bans", a.ginAdminGetPlayerDataBans)
				admin.GET("/player-data/reports", a.ginAdminGetPlayerDataReports)
				admin.GET("/player-data/stats", a.ginAdminGetPlayerDataStats)
			}
		}
	}

	return router
}
