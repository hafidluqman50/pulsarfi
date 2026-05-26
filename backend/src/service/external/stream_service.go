package external

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
)

type StreamService struct {
	b *broadcaster
}

func NewStreamService() *StreamService {
	return &StreamService{b: newBroadcaster()}
}

func (s *StreamService) Emit(level EventLevel, tag, message string) {
	s.b.emit(newStreamEvent(level, tag, message))
}

func (s *StreamService) Stream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ch, unsubscribe := s.b.subscribe()
	defer unsubscribe()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
		}
	}
}
