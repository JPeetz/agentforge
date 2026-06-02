// handlers_api.go — Week 1 API endpoints: auth, cost, chat/SSE stream.
package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/agentforge/agentforge/internal/auth"
)

// ── Auth API ─────────────────────────────────────────────────────────────────

func (s *Server) handleLoginAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if s.authStore == nil || s.authManager == nil {
		// Fallback: accept any password (backward compat for testing)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"accessToken":"dev-token","refreshToken":"dev-refresh","user":{"id":"admin-001","username":"admin","role":"admin"}}`))
		return
	}
	user, err := s.authStore.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid credentials"}`))
		return
	}
	access, refresh, err := s.authManager.GenerateTokenPair(user.ID, user.Role)
	if err != nil {
		http.Error(w, `{"error":"token generation failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"accessToken":  access,
		"refreshToken": refresh,
		"user":         map[string]any{"id": user.ID, "username": user.Username, "role": user.Role},
	})
}

func (s *Server) handleRefreshAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct{ RefreshToken string `json:"refreshToken"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if s.authManager == nil {
		http.Error(w, `{"error":"auth not configured"}`, http.StatusInternalServerError)
		return
	}
	access, refresh, err := s.authManager.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid refresh token"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"accessToken": access, "refreshToken": refresh})
}

func (s *Server) handleMeAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"username": "admin", "role": "admin"})
}

func (s *Server) handleAPIKeyAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.authStore == nil {
		http.Error(w, `{"error":"auth store not available"}`, http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.Write([]byte(`[]`))
	case http.MethodPost:
		apiKey, rawKey, err := s.authStore.GenerateAPIKey("admin-001", nil, auth.RoleOperator, 30*24*time.Hour)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"apiKey": apiKey, "rawKey": rawKey})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ── Cost API ────────────────────────────────────────────────────────────────

func (s *Server) handleCostSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.costTracker == nil {
		w.Write([]byte(`{"totalCost":0,"totalTokens":{"prompt":0,"completion":0,"total":0}}`))
		return
	}
	json.NewEncoder(w).Encode(s.costTracker.Overview())
}

func (s *Server) handleCostDaily(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.costTracker == nil {
		w.Write([]byte(`[]`))
		return
	}
	json.NewEncoder(w).Encode(s.costTracker.GetDailyCosts(7))
}

func (s *Server) handleCostBudget(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.costTracker == nil {
		w.Write([]byte(`{"spent":0,"limit":0,"percent":0,"level":"ok"}`))
		return
	}
	spent, limit, percent, level := s.costTracker.BudgetStatus("*")
	json.NewEncoder(w).Encode(map[string]any{"spent": spent, "limit": limit, "percent": percent, "level": level})
}

func (s *Server) handleCostModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.costTracker == nil {
		w.Write([]byte(`[]`))
		return
	}
	json.NewEncoder(w).Encode(s.costTracker.GetModelCosts())
}

func (s *Server) handleCostProviders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.costTracker == nil {
		w.Write([]byte(`[]`))
		return
	}
	json.NewEncoder(w).Encode(s.costTracker.GetProviderCosts())
}

// ── Chat Stream (SSE) ───────────────────────────────────────────────────────

