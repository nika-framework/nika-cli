package src

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nika-framework/nika/common/microservice"
)

// AppController is the message surface of the {{.AppName}} service.
//
// Every field declares the transport it is served on and the subject clients
// address. A field may also carry a `route` tag, in which case the same handler
// serves HTTP and messages — see microservice.IsMessage.
// Patterns use '_' rather than '.' as their separator: a dot is AMQP's topic
// word separator, so RabbitMQ refuses a pattern containing one, and '_' is the
// one separator every transport here accepts.
type AppController struct {
	Ping func(*gin.Context) `transport:"{{.Transport}}" pattern:"{{.Subject}}_ping"`
	Echo func(*gin.Context) `transport:"{{.Transport}}" pattern:"{{.Subject}}_echo"`
}

func NewAppController() *AppController {
	return &AppController{
		// Send it with:
		//   client.Send(ctx, "{{.Subject}}_ping", nil, &reply)
		Ping: func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"service":   "{{.AppName}}",
					"transport": "{{.Transport}}",
					"pattern":   microservice.PatternFrom(c),
				},
			})
		},

		// Echo shows how to read a payload. ShouldBindJSON works over a message
		// transport because the envelope's data is handed to the handler as the
		// request body.
		Echo: func(c *gin.Context) {
			var payload map[string]any
			if err := c.ShouldBindJSON(&payload); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   gin.H{"code": 400, "message": "INVALID_PAYLOAD"},
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": payload})
		},
	}
}
