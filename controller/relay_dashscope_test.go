package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestPrepareDashScopeInferenceRelay_SelectsChannelFromFirstFrame(t *testing.T) {
	db := setupEndpointFilterTestDB(t)
	origMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = origMemoryCache })

	channel := insertTestChannel(t, db, "ali-inference", "sambert-zhichu-v1", "default", constant.ChannelTypeAli)

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	connCh := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		connCh <- conn
	}))
	defer server.Close()

	clientConn, _, err := websocket.DefaultDialer.Dial("ws"+server.URL[len("http"):], nil)
	assert.NoError(t, err)
	defer clientConn.Close()
	serverConn := <-connCh
	defer serverConn.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api-ws/v1/inference", nil)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")

	message := []byte(`{"header":{"action":"run-task","task_id":"task-1"},"payload":{"model":"sambert-zhichu-v1","input":{"text":"你好"}}}`)
	assert.NoError(t, clientConn.WriteMessage(websocket.TextMessage, message))
	apiErr := prepareDashScopeInferenceRelay(c, serverConn)

	assert.Nil(t, apiErr)
	assert.False(t, c.IsAborted())
	assert.Equal(t, "sambert-zhichu-v1", c.Query("model"))
	assert.Equal(t, channel.Id, common.GetContextKeyInt(c, constant.ContextKeyChannelId))
	assert.Equal(t, constant.ChannelTypeAli, common.GetContextKeyInt(c, constant.ContextKeyChannelType))
	firstMessage, ok := c.Get("first_wss_request")
	assert.True(t, ok)
	assert.Equal(t, message, firstMessage)
	assert.Equal(t, websocket.TextMessage, c.GetInt("first_wss_request_type"))
}
