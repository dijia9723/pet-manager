package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// 覆盖凭证提取的优先级与边界：仅 WebSocket Upgrade 请求才认子协议承载的 token，
// 普通请求忽略子协议，Authorization 头优先于子协议与 query。
func TestWebSocketTokenIsTakenFromSubprotocolOnlyOnUpgrade(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, upgrade, protocols, authorization, query, want string
	}{
		{"browser websocket", "websocket", "pet-manager-terminal.v1, bearer.test-jwt", "", "", "test-jwt"},
		{"normal request ignores subprotocol", "", "bearer.test-jwt", "", "", ""},
		{"header takes precedence", "websocket", "bearer.test-jwt", "Bearer header-jwt", "", "header-jwt"},
		{"no credential", "websocket", "pet-manager-terminal.v1", "", "", ""},
		{"existing query support", "", "", "", "?access_token=query-jwt", "query-jwt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/api/v1/platform/servers/1/terminal"+tc.query, nil)
			c.Request.Header.Set("Upgrade", tc.upgrade)
			c.Request.Header.Set("Sec-WebSocket-Protocol", tc.protocols)
			c.Request.Header.Set("Authorization", tc.authorization)
			if got := extractToken(c); got != tc.want {
				t.Fatalf("got %q; want %q", got, tc.want)
			}
		})
	}
}
