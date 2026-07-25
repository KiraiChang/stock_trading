package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
)

type positionRepoStub struct {
	applyErr error
}

func (s *positionRepoStub) List(context.Context, uint64) ([]store.Position, error) {
	return nil, nil
}
func (s *positionRepoStub) Get(context.Context, uint64, string) (*store.Position, error) {
	return nil, nil
}
func (s *positionRepoStub) ListTransactions(context.Context, uint64, string, int) ([]store.PositionTransaction, error) {
	return nil, nil
}
func (s *positionRepoStub) ApplyEvent(context.Context, uint64, *store.PositionTransaction, int64) (*store.Position, error) {
	return nil, s.applyErr
}
func (s *positionRepoStub) CreateAnalysis(context.Context, *store.PositionAnalysis) (uint64, error) {
	return 0, nil
}
func (s *positionRepoStub) GetAnalysis(context.Context, uint64, uint64) (*store.PositionAnalysis, error) {
	return nil, nil
}
func (s *positionRepoStub) ListAnalyses(context.Context, uint64, string, int) ([]store.PositionAnalysis, error) {
	return nil, nil
}

func TestPositionApplyEventErrorMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		path       string
		body       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "transaction validation error",
			path:       "/positions/2330/transactions",
			body:       `{"portfolio_id":2,"event_type":"SELL","shares":10,"price":100}`,
			err:        errors.Join(store.ErrPositionInvalidEvent, errors.New("SELL shares exceed current position")),
			wantStatus: http.StatusBadRequest,
			wantBody:   "SELL shares exceed current position",
		},
		{
			name:       "adjustment infrastructure error",
			path:       "/positions/2330/adjustments",
			body:       `{"portfolio_id":2,"target_shares":10,"target_avg_cost":100,"reason":"reconcile"}`,
			err:        errors.New("driver: bad connection secret"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "internal server error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &positionRepoStub{applyErr: tt.err}
			h := NewPositionHandler(repo, nil, zap.NewNop())
			router := gin.New()
			router.POST("/positions/:symbol/transactions", h.AddTransaction)
			router.POST("/positions/:symbol/adjustments", h.Adjust)

			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus || !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("status/body = %d %s, want %d containing %q", rec.Code, rec.Body.String(), tt.wantStatus, tt.wantBody)
			}
			if tt.wantStatus == http.StatusInternalServerError && strings.Contains(rec.Body.String(), "secret") {
				t.Fatalf("infrastructure details leaked to client: %s", rec.Body.String())
			}
		})
	}
}

func TestPositionRequiresPortfolioID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewPositionHandler(&positionRepoStub{}, nil, zap.NewNop())
	router := gin.New()
	router.GET("/positions", h.List)
	router.POST("/positions/:symbol/transactions", h.AddTransaction)

	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/positions", nil))
	if listRec.Code != http.StatusBadRequest || !strings.Contains(listRec.Body.String(), "portfolio_id is required") {
		t.Fatalf("unexpected list response: %d %s", listRec.Code, listRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/positions/2330/transactions", bytes.NewBufferString(`{"event_type":"BUY","shares":10,"price":100}`))
	req.Header.Set("Content-Type", "application/json")
	writeRec := httptest.NewRecorder()
	router.ServeHTTP(writeRec, req)
	if writeRec.Code != http.StatusBadRequest || !strings.Contains(writeRec.Body.String(), "portfolio_id is required") {
		t.Fatalf("unexpected write response: %d %s", writeRec.Code, writeRec.Body.String())
	}
}
