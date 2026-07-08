package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBindHoldingRequestValidationAndNormalization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		wantOK     bool
		wantStatus int
		wantSymbol string
		wantNote   string
	}{
		{
			name:       "valid request normalizes symbol and note",
			body:       `{"symbol":" 2330 ","shares":1000,"cost_price":600,"note":" core "}`,
			wantOK:     true,
			wantStatus: http.StatusOK,
			wantSymbol: "2330",
			wantNote:   "core",
		},
		{
			name:       "symbol is required",
			body:       `{"symbol":" ","shares":1000,"cost_price":600}`,
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "shares must be positive",
			body:       `{"symbol":"2330","shares":0,"cost_price":600}`,
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "cost price must be positive",
			body:       `{"symbol":"2330","shares":1000,"cost_price":0}`,
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/holdings", strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			req, ok := bindHoldingRequest(c)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d body=%s", tt.wantStatus, w.Code, w.Body.String())
			}
			if ok && (req.Symbol != tt.wantSymbol || req.Note != tt.wantNote) {
				t.Fatalf("unexpected normalized request: %+v", req)
			}
		})
	}
}
