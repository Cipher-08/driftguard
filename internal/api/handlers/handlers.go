package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/driftguard/driftguard/internal/api/middleware"
	"github.com/driftguard/driftguard/internal/config"
	"github.com/driftguard/driftguard/internal/engine"
	"github.com/driftguard/driftguard/internal/llm"
	"github.com/driftguard/driftguard/internal/models"
	"github.com/driftguard/driftguard/internal/remediation"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// ScanFunc triggers an on-demand collection across all configured providers.
type ScanFunc func(ctx context.Context) error

type Handler struct {
	cfg       *config.Config
	db        *pgxpool.Pool
	engine    *engine.Engine
	prCreator *remediation.PRCreator
	llm       llm.Client // nil if no provider configured
	runScan   ScanFunc   // nil if no collectors wired
	logger    *zap.Logger
}

func New(cfg *config.Config, db *pgxpool.Pool, eng *engine.Engine, llmClient llm.Client, runScan ScanFunc, logger *zap.Logger) *Handler {
	return &Handler{
		cfg:       cfg,
		db:        db,
		engine:    eng,
		prCreator: remediation.NewPRCreator(cfg.GitHubToken),
		llm:       llmClient,
		runScan:   runScan,
		logger:    logger,
	}
}

// ---- Auth ----

type registerRequest struct {
	OrgName  string `json:"org_name" binding:"required"`
	OrgSlug  string `json:"org_slug" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	tx, err := h.db.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to begin transaction"})
		return
	}
	defer tx.Rollback(c.Request.Context())

	var orgID uuid.UUID
	err = tx.QueryRow(c.Request.Context(), `
		INSERT INTO organizations (name, slug) VALUES ($1, $2) RETURNING id
	`, req.OrgName, req.OrgSlug).Scan(&orgID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "org slug already taken"})
		return
	}

	var userID uuid.UUID
	err = tx.QueryRow(c.Request.Context(), `
		INSERT INTO users (org_id, email, password_hash, name, role)
		VALUES ($1, $2, $3, $4, 'admin')
		RETURNING id
	`, orgID, req.Email, string(hash), req.Name).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to commit"})
		return
	}

	token, err := middleware.GenerateToken(userID, orgID, req.Email, "admin", h.cfg.JWTSecret, h.cfg.JWTExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user":  gin.H{"id": userID, "email": req.Email, "name": req.Name, "role": "admin"},
	})
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, org_id, email, password_hash, name, role FROM users WHERE email = $1
	`, req.Email).Scan(&user.ID, &user.OrgID, &user.Email, &user.PasswordHash, &user.Name, &user.Role)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := middleware.GenerateToken(user.ID, user.OrgID, user.Email, user.Role, h.cfg.JWTSecret, h.cfg.JWTExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  gin.H{"id": user.ID, "email": user.Email, "name": user.Name, "role": user.Role},
	})
}

// ---- Dashboard ----

func (h *Handler) GetSummary(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	summary, err := h.engine.GetDriftSummary(c.Request.Context(), orgID)
	if err != nil {
		h.logger.Error("get summary failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get summary"})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// ---- Cloud Credentials ----

type addCredentialRequest struct {
	Provider    string                 `json:"provider" binding:"required"` // aws, gcp, azure
	Name        string                 `json:"name" binding:"required"`
	Credentials map[string]interface{} `json:"credentials" binding:"required"`
}

// AddCredential stores cloud provider credentials for the org.
func (h *Handler) AddCredential(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	var req addCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	credJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credentials format"})
		return
	}

	var id uuid.UUID
	err = h.db.QueryRow(c.Request.Context(),
		`INSERT INTO cloud_credentials (org_id, provider, name, credentials, is_active)
		 VALUES ($1, $2, $3, $4, true) RETURNING id`,
		orgID, req.Provider, req.Name, credJSON,
	).Scan(&id)
	if err != nil {
		h.logger.Error("store credentials failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store credentials"})
		return
	}

	// Kick off a scan in the background so the user sees data quickly.
	if h.runScan != nil {
		go func() {
			if err := h.runScan(context.Background()); err != nil {
				h.logger.Error("post-credential scan failed", zap.Error(err))
			}
		}()
	}

	c.JSON(http.StatusCreated, gin.H{"id": id, "provider": req.Provider, "name": req.Name})
}

// ListCredentials returns all connected cloud accounts for the org.
func (h *Handler) ListCredentials(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	rows, err := h.db.Query(c.Request.Context(),
		`SELECT id, provider, name, is_active, created_at FROM cloud_credentials WHERE org_id = $1 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list credentials"})
		return
	}
	defer rows.Close()

	accounts := []gin.H{}
	for rows.Next() {
		var cred models.CloudCredential
		if err := rows.Scan(&cred.ID, &cred.Provider, &cred.Name, &cred.IsActive, &cred.CreatedAt); err != nil {
			continue
		}
		accounts = append(accounts, gin.H{
			"id": cred.ID, "provider": cred.Provider, "name": cred.Name,
			"is_active": cred.IsActive, "created_at": cred.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"accounts": accounts, "count": len(accounts)})
}

// DeleteCredential removes a connected cloud account.
func (h *Handler) DeleteCredential(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credential id"})
		return
	}
	tag, err := h.db.Exec(c.Request.Context(),
		`DELETE FROM cloud_credentials WHERE id = $1 AND org_id = $2`, id, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete credential"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "credential not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "credential removed"})
}

// ---- Scan ----

// TriggerScan runs an on-demand collection across all providers.
func (h *Handler) TriggerScan(c *gin.Context) {
	if h.runScan == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no collectors configured"})
		return
	}
	go func() {
		if err := h.runScan(context.Background()); err != nil {
			h.logger.Error("manual scan failed", zap.Error(err))
		}
	}()
	c.JSON(http.StatusAccepted, gin.H{"message": "scan started"})
}

// ---- Drift ----

func (h *Handler) ListDrifts(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	resolved := c.DefaultQuery("resolved", "false")
	severity := c.DefaultQuery("severity", "")

	query := `
		SELECT d.id, d.org_id, d.resource_id, d.drift_type, d.severity, d.diff,
		       d.changed_by, d.changed_at, d.is_resolved, d.resolved_at, d.created_at,
		       r.provider, r.region, r.resource_type, r.resource_id, r.resource_name
		FROM drift_records d
		JOIN resources r ON r.id = d.resource_id
		WHERE d.org_id = $1 AND d.is_resolved = $2
	`
	args := []interface{}{orgID, resolved == "true"}

	if severity != "" {
		query += fmt.Sprintf(" AND d.severity = $%d", len(args)+1)
		args = append(args, severity)
	}
	query += " ORDER BY d.created_at DESC LIMIT 100"

	rows, err := h.db.Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list drifts"})
		return
	}
	defer rows.Close()

	drifts := []gin.H{}
	for rows.Next() {
		var d models.DriftRecord
		var r models.Resource
		err := rows.Scan(
			&d.ID, &d.OrgID, &d.ResourceID, &d.DriftType, &d.Severity, &d.Diff,
			&d.ChangedBy, &d.ChangedAt, &d.IsResolved, &d.ResolvedAt, &d.CreatedAt,
			&r.Provider, &r.Region, &r.ResourceType, &r.ResourceID, &r.ResourceName,
		)
		if err != nil {
			continue
		}
		drifts = append(drifts, gin.H{
			"id": d.ID, "drift_type": d.DriftType, "severity": d.Severity,
			"diff": d.Diff, "changed_by": d.ChangedBy, "changed_at": d.ChangedAt,
			"is_resolved": d.IsResolved, "resolved_at": d.ResolvedAt, "created_at": d.CreatedAt,
			"resource": gin.H{
				"id": d.ResourceID, "provider": r.Provider, "region": r.Region,
				"resource_type": r.ResourceType, "resource_id": r.ResourceID, "resource_name": r.ResourceName,
			},
		})
	}
	c.JSON(http.StatusOK, gin.H{"drifts": drifts, "count": len(drifts)})
}

// GetDrift returns a single drift record with its violations and remediations.
func (h *Handler) GetDrift(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	driftID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid drift id"})
		return
	}

	var d models.DriftRecord
	var r models.Resource
	err = h.db.QueryRow(c.Request.Context(), `
		SELECT d.id, d.resource_id, d.drift_type, d.severity, d.diff, d.changed_by,
		       d.changed_at, d.is_resolved, d.created_at,
		       r.provider, r.region, r.resource_type, r.resource_id, r.resource_name, r.live_state, r.declared_state
		FROM drift_records d
		JOIN resources r ON r.id = d.resource_id
		WHERE d.id = $1 AND d.org_id = $2
	`, driftID, orgID).Scan(
		&d.ID, &d.ResourceID, &d.DriftType, &d.Severity, &d.Diff, &d.ChangedBy,
		&d.ChangedAt, &d.IsResolved, &d.CreatedAt,
		&r.Provider, &r.Region, &r.ResourceType, &r.ResourceID, &r.ResourceName, &r.LiveState, &r.DeclaredState,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "drift not found"})
		return
	}

	violations := h.loadViolations(c, d.ResourceID)
	remediations := h.loadRemediations(c, driftID)

	c.JSON(http.StatusOK, gin.H{
		"id": d.ID, "drift_type": d.DriftType, "severity": d.Severity,
		"diff": d.Diff, "changed_by": d.ChangedBy, "changed_at": d.ChangedAt,
		"is_resolved": d.IsResolved, "created_at": d.CreatedAt,
		"resource": gin.H{
			"id": d.ResourceID, "provider": r.Provider, "region": r.Region,
			"resource_type": r.ResourceType, "resource_id": r.ResourceID,
			"resource_name": r.ResourceName, "live_state": r.LiveState, "declared_state": r.DeclaredState,
		},
		"violations":   violations,
		"remediations": remediations,
	})
}

func (h *Handler) loadViolations(c *gin.Context, resourceID uuid.UUID) []gin.H {
	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, policy_id, policy_name, framework, description, severity, created_at
		FROM compliance_violations WHERE resource_id = $1 ORDER BY severity, created_at DESC
	`, resourceID)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var v models.ComplianceViolation
		if err := rows.Scan(&v.ID, &v.PolicyID, &v.PolicyName, &v.Framework, &v.Description, &v.Severity, &v.CreatedAt); err != nil {
			continue
		}
		out = append(out, gin.H{
			"id": v.ID, "policy_id": v.PolicyID, "policy_name": v.PolicyName,
			"framework": v.Framework, "description": v.Description, "severity": v.Severity,
		})
	}
	return out
}

func (h *Handler) loadRemediations(c *gin.Context, driftID uuid.UUID) []gin.H {
	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, patch, pr_url, pr_number, status, created_at
		FROM remediations WHERE drift_id = $1 ORDER BY created_at DESC
	`, driftID)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var rem models.Remediation
		var prURL *string
		var prNumber *int
		if err := rows.Scan(&rem.ID, &rem.Patch, &prURL, &prNumber, &rem.Status, &rem.CreatedAt); err != nil {
			continue
		}
		out = append(out, gin.H{
			"id": rem.ID, "patch": rem.Patch, "pr_url": prURL, "pr_number": prNumber, "status": rem.Status,
		})
	}
	return out
}

func (h *Handler) ResolveDrift(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	userID := middleware.GetUserID(c)
	driftID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid drift id"})
		return
	}
	_, err = h.db.Exec(c.Request.Context(), `
		UPDATE drift_records SET is_resolved = true, resolved_at = NOW(), resolved_by = $1
		WHERE id = $2 AND org_id = $3
	`, userID, driftID, orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve drift"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "drift resolved"})
}

// ---- Remediation (AI) ----

// GenerateRemediation uses the configured LLM to produce a Terraform patch for a drift.
func (h *Handler) GenerateRemediation(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	driftID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid drift id"})
		return
	}

	if h.llm == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "no AI provider configured — set GROQ_API_KEY, GEMINI_API_KEY, or OLLAMA_HOST",
		})
		return
	}

	var (
		resourceType, resourceID, severity string
		liveState, declaredState, diff     []byte
	)
	err = h.db.QueryRow(c.Request.Context(), `
		SELECT r.resource_type, r.resource_id, d.severity, r.live_state, r.declared_state, d.diff
		FROM drift_records d JOIN resources r ON r.id = d.resource_id
		WHERE d.id = $1 AND d.org_id = $2
	`, driftID, orgID).Scan(&resourceType, &resourceID, &severity, &liveState, &declaredState, &diff)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "drift not found"})
		return
	}

	patch, err := h.llm.GeneratePatch(c.Request.Context(), llm.PatchRequest{
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		Severity:      severity,
		LiveState:     liveState,
		DeclaredState: declaredState,
		Diff:          diff,
	})
	if err != nil {
		h.logger.Error("llm patch generation failed", zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to generate remediation: " + err.Error()})
		return
	}

	var remID uuid.UUID
	err = h.db.QueryRow(c.Request.Context(), `
		INSERT INTO remediations (org_id, drift_id, patch, status)
		VALUES ($1, $2, $3, 'pending') RETURNING id
	`, orgID, driftID, patch).Scan(&remID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save remediation"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": remID, "patch": patch, "status": "pending"})
}

type openPRRequest struct {
	RepoOwner  string `json:"repo_owner" binding:"required"`
	RepoName   string `json:"repo_name" binding:"required"`
	BaseBranch string `json:"base_branch" binding:"required"`
	FilePath   string `json:"file_path" binding:"required"`
}

func (h *Handler) OpenPR(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	driftID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid drift id"})
		return
	}
	remID, err := uuid.Parse(c.Param("rem_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid remediation id"})
		return
	}
	var req openPRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.cfg.GitHubToken == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "GITHUB_TOKEN not configured"})
		return
	}

	var patch, resourceName, severity string
	err = h.db.QueryRow(c.Request.Context(), `
		SELECT rem.patch, r.resource_name, d.severity
		FROM remediations rem
		JOIN drift_records d ON d.id = rem.drift_id
		JOIN resources r ON r.id = d.resource_id
		WHERE rem.id = $1 AND rem.drift_id = $2 AND rem.org_id = $3
	`, remID, driftID, orgID).Scan(&patch, &resourceName, &severity)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "remediation not found"})
		return
	}

	result, err := h.prCreator.OpenPR(c.Request.Context(), remediation.PRRequest{
		RepoOwner: req.RepoOwner, RepoName: req.RepoName, BaseBranch: req.BaseBranch,
		FilePath: req.FilePath, Patch: patch, DriftID: driftID,
		ResourceName: resourceName, Severity: severity,
	})
	if err != nil {
		h.logger.Error("failed to open PR", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if _, err := h.db.Exec(c.Request.Context(), `
		UPDATE remediations SET status='pr_opened', pr_url=$1, pr_number=$2, updated_at=NOW() WHERE id=$3
	`, result.PRURL, result.PRNumber, remID); err != nil {
		h.logger.Error("failed to update remediation", zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{"pr_url": result.PRURL, "pr_number": result.PRNumber})
}

// ---- Resources ----

func (h *Handler) ListResources(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	provider := c.DefaultQuery("provider", "")

	query := `SELECT id, provider, region, resource_type, resource_id, resource_name, last_scanned_at FROM resources WHERE org_id = $1`
	args := []interface{}{orgID}
	if provider != "" {
		query += " AND provider = $2"
		args = append(args, provider)
	}
	query += " ORDER BY last_scanned_at DESC LIMIT 200"

	rows, err := h.db.Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list resources"})
		return
	}
	defer rows.Close()

	resources := []gin.H{}
	for rows.Next() {
		var r models.Resource
		if err := rows.Scan(&r.ID, &r.Provider, &r.Region, &r.ResourceType, &r.ResourceID, &r.ResourceName, &r.LastScannedAt); err != nil {
			continue
		}
		resources = append(resources, gin.H{
			"id": r.ID, "provider": r.Provider, "region": r.Region,
			"resource_type": r.ResourceType, "resource_id": r.ResourceID,
			"resource_name": r.ResourceName, "last_scanned_at": r.LastScannedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"resources": resources, "count": len(resources)})
}

func (h *Handler) Health(c *gin.Context) {
	if err := h.db.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":       "healthy",
		"ai_enabled":   h.llm != nil,
		"ai_provider":  h.cfg.LLMProviderName(),
		"github_ready": h.cfg.GitHubToken != "",
	})
}
