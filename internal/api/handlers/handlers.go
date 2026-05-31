// ListCredentials returns all connected cloud accounts for the org
func (h *Handler) ListCredentials(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	rows, err := h.db.Query(c.Request.Context(),
		`SELECT id, provider, name, is_active, created_at FROM cloud_credentials WHERE org_id = $1 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to list credentials"})
		return
	}
	defer rows.Close()
	creds := []gin.H{}
	for rows.Next() {
		var id, provider, name string
		var isActive bool
		var createdAt string
		if err := rows.Scan(&id, &provider, &name, &isActive, &createdAt); err != nil {
			continue
		}
		creds = append(creds, gin.H{
			"id": id,
			"provider": provider,
			"name": name,
			"is_active": isActive,
			"created_at": createdAt,
		})
	}
	c.JSON(200, gin.H{"accounts": creds})
}
package handlers

import (
	"fmt"
	"net/http"

	"github.com/driftguard/driftguard/internal/api/middleware"
	"github.com/driftguard/driftguard/internal/config"
	"github.com/driftguard/driftguard/internal/engine"
	"github.com/driftguard/driftguard/internal/models"
	"github.com/driftguard/driftguard/internal/remediation"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	cfg       *config.Config
	db        *pgxpool.Pool
	engine    *engine.Engine
	prCreator *remediation.PRCreator
	logger    *zap.Logger
}

func New(cfg *config.Config, db *pgxpool.Pool, eng *engine.Engine, logger *zap.Logger) *Handler {
       return &Handler{
	       cfg:       cfg,
	       db:        db,
	       engine:    eng,
	       prCreator: remediation.NewPRCreator(cfg.GitHubToken),
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

	c.JSON(http.StatusCreated, gin.H{"token": token, "org_id": orgID, "user_id": userID})
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

// ---- Drift ----

func (h *Handler) GetSummary(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	summary, err := h.engine.GetDriftSummary(c.Request.Context(), orgID)
	if err != nil {
		h.logger.Error("get summary failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get summary"})
		return
	}
// ---- Cloud Credentials ----

	c.JSON(http.StatusOK, summary)
}

func (h *Handler) ListDrifts(c *gin.Context) {
	orgID := middleware.GetOrgID(c)

// AddCredential allows users to add cloud credentials (GCP, AWS, Azure)
func (h *Handler) AddCredential(c *gin.Context) {
	orgID := middleware.GetOrgID(c)
	var req addCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Store credentials as JSON
	credJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credentials format"})
		return
	}
	_, err = h.db.Exec(c.Request.Context(),
		`INSERT INTO cloud_credentials (org_id, provider, name, credentials, is_active, created_at)
		 VALUES ($1, $2, $3, $4, true, NOW())`,
		orgID, req.Provider, req.Name, credJSON,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store credentials"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
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
	query += fmt.Sprintf(" ORDER BY d.created_at DESC LIMIT 100")

	rows, err := h.db.Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list drifts"})
		return
	}
	defer rows.Close()

	var drifts []gin.H
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
	if drifts == nil {
		drifts = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"drifts": drifts, "count": len(drifts)})
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



type openPRRequest struct {
	RepoOwner  string `json:"repo_owner" binding:"required"`
	RepoName   string `json:"repo_name" binding:"required"`
	BaseBranch string `json:"base_branch" binding:"required"`
	FilePath   string `json:"file_path" binding:"required"`
}

func (h *Handler) OpenPR(c *gin.Context) {
	driftID, err := uuid.Parse(c.Param("drift_id"))
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

	var patch, resourceName, severity string
	err = h.db.QueryRow(c.Request.Context(), `
		SELECT rem.patch, r.resource_name, d.severity
		FROM remediations rem
		JOIN drift_records d ON d.id = rem.drift_id
		JOIN resources r ON r.id = d.resource_id
		WHERE rem.id = $1 AND rem.drift_id = $2
	`, remID, driftID).Scan(&patch, &resourceName, &severity)
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

	var resources []gin.H
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
	if resources == nil {
		resources = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"resources": resources, "count": len(resources)})
}

func (h *Handler) Health(c *gin.Context) {
	if err := h.db.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}
