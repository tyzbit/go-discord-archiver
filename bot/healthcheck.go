package bot

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (b ArchiverBot) StartHealthAPI() {
	app := gin.New()
	app.Use(
		// Disable logging for healthcheck endpoint and favicon
		gin.LoggerWithWriter(gin.DefaultWriter, "/healthcheck", "/favicon.ico"),
		gin.Recovery(),
	)
	app.GET("/healthcheck", func(c *gin.Context) {
		status := http.StatusOK
		content := "Healthy"
		rawDb, err := b.DB.DB()
		pingResult := rawDb.Ping()
		if err != nil {
			content = fmt.Sprintf("Error opening db: %v", err)
			status = http.StatusInternalServerError
		}
		if pingResult != nil {
			content = fmt.Sprintf("Error pinging db: %v", pingResult)
			status = http.StatusInternalServerError
		}
		c.String(status, content)
	})
	_ = app.Run(":8080")
}
