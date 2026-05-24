package response

import "github.com/gin-gonic/gin"

type Body struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Data       any    `json:"data"`
}

func OK(c *gin.Context, message string, data any) {
	c.JSON(200, Body{StatusCode: 200, Message: message, Data: data})
}

func Created(c *gin.Context, message string, data any) {
	c.JSON(201, Body{StatusCode: 201, Message: message, Data: data})
}

func BadRequest(c *gin.Context, message string) {
	c.AbortWithStatusJSON(400, Body{StatusCode: 400, Message: message, Data: nil})
}

func Unauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(401, Body{StatusCode: 401, Message: message, Data: nil})
}

func Forbidden(c *gin.Context, message string) {
	c.AbortWithStatusJSON(403, Body{StatusCode: 403, Message: message, Data: nil})
}

func NotFound(c *gin.Context, message string) {
	c.AbortWithStatusJSON(404, Body{StatusCode: 404, Message: message, Data: nil})
}

func UnprocessableEntity(c *gin.Context, message string, data any) {
	c.AbortWithStatusJSON(422, Body{StatusCode: 422, Message: message, Data: data})
}

func InternalError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(500, Body{StatusCode: 500, Message: message, Data: nil})
}
