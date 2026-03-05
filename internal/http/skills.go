package http

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/permissions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/store/pg"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

const maxSkillUploadSize = 20 << 20 // 20 MB

var slugRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// SkillsHandler handles skill management HTTP endpoints (managed mode).
type SkillsHandler struct {
	skills  *pg.PGSkillStore
	baseDir string // filesystem base for skill content
	token   string
	msgBus  *bus.MessageBus
}

// NewSkillsHandler creates a handler for skill management endpoints.
func NewSkillsHandler(skills *pg.PGSkillStore, baseDir, token string, msgBus *bus.MessageBus) *SkillsHandler {
	return &SkillsHandler{skills: skills, baseDir: baseDir, token: token, msgBus: msgBus}
}

// emitCacheInvalidate broadcasts a cache invalidation event if msgBus is set.
func (h *SkillsHandler) emitCacheInvalidate(kind, key string) {
	if h.msgBus == nil {
		return
	}
	h.msgBus.Broadcast(bus.Event{
		Name:    protocol.EventCacheInvalidate,
		Payload: bus.CacheInvalidatePayload{Kind: kind, Key: key},
	})
}

// RegisterRoutes registers all skill management routes on the given mux.
func (h *SkillsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/skills", requireRoleHTTP(h.token, permissions.RoleViewer, h.handleList))
	mux.HandleFunc("GET /v1/skills/{id}", requireRoleHTTP(h.token, permissions.RoleViewer, h.handleGet))
	mux.HandleFunc("GET /v1/agents/{agentID}/skills", requireRoleHTTP(h.token, permissions.RoleViewer, h.handleListAgentSkills))

	mux.HandleFunc("POST /v1/skills/upload", requireRoleHTTP(h.token, permissions.RoleOperator, h.handleUpload))
	mux.HandleFunc("PUT /v1/skills/{id}", requireRoleHTTP(h.token, permissions.RoleOperator, h.handleUpdate))
	mux.HandleFunc("DELETE /v1/skills/{id}", requireRoleHTTP(h.token, permissions.RoleOperator, h.handleDelete))
	mux.HandleFunc("POST /v1/skills/{id}/grants/agent", requireRoleHTTP(h.token, permissions.RoleOperator, h.handleGrantAgent))
	mux.HandleFunc("DELETE /v1/skills/{id}/grants/agent/{agentID}", requireRoleHTTP(h.token, permissions.RoleOperator, h.handleRevokeAgent))
	mux.HandleFunc("POST /v1/skills/{id}/grants/user", requireRoleHTTP(h.token, permissions.RoleOperator, h.handleGrantUser))
	mux.HandleFunc("DELETE /v1/skills/{id}/grants/user/{userID}", requireRoleHTTP(h.token, permissions.RoleOperator, h.handleRevokeUser))
}

func (h *SkillsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	skills := h.skills.ListSkills()
	writeJSON(w, http.StatusOK, map[string]interface{}{"skills": skills})
}

func (h *SkillsHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	skill, ok := h.skills.GetSkill(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "skill not found"})
		return
	}
	writeJSON(w, http.StatusOK, skill)
}

func (h *SkillsHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill ID"})
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	// Prevent changing sensitive fields
	delete(updates, "id")
	delete(updates, "owner_id")
	delete(updates, "file_path")

	if err := h.skills.UpdateSkill(id, updates); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.skills.BumpVersion()
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (h *SkillsHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill ID"})
		return
	}

	if err := h.skills.DeleteSkill(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.skills.BumpVersion()
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// handleUpload processes a ZIP file upload containing a skill (must have SKILL.md at root).
func (h *SkillsHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	userID := store.UserIDFromContext(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-GoClaw-User-Id header required"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSkillUploadSize)

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required: " + err.Error()})
		return
	}
	defer file.Close()

	// Save to temp file for zip processing
	tmp, err := os.CreateTemp("", "skill-upload-*.zip")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create temp file"})
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hasher), file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save upload"})
		return
	}
	fileHash := fmt.Sprintf("%x", hasher.Sum(nil))

	// Open as zip
	zr, err := zip.OpenReader(tmp.Name())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ZIP file"})
		return
	}
	defer zr.Close()

	// Validate: must have SKILL.md at root
	var skillMD *zip.File
	for _, f := range zr.File {
		if f.Name == "SKILL.md" || f.Name == "./SKILL.md" {
			skillMD = f
			break
		}
	}
	if skillMD == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ZIP must contain SKILL.md at root"})
		return
	}

	// Read and parse SKILL.md frontmatter
	skillContent, err := readZipFile(skillMD)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read SKILL.md"})
		return
	}

	name, description, slug := parseSkillFrontmatter(skillContent)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "SKILL.md must have a name in frontmatter"})
		return
	}
	if slug == "" {
		slug = slugify(name)
	}
	if !slugRegexp.MatchString(slug) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slug must be a valid slug (lowercase letters, numbers, hyphens only)"})
		return
	}

	// Determine version (increment if slug already exists)
	version := 1
	if existing, ok := h.skills.GetSkill(slug); ok {
		_ = existing
		version = h.skills.GetNextVersion(slug)
	}

	// Extract to filesystem: baseDir/slug/version/
	destDir := filepath.Join(h.baseDir, slug, fmt.Sprintf("%d", version))
	if err := os.MkdirAll(destDir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create skill directory"})
		return
	}

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// Security: prevent path traversal
		name := filepath.Clean(f.Name)
		if strings.Contains(name, "..") {
			continue
		}
		destPath := filepath.Join(destDir, name)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			continue
		}
		data, err := readZipFile(f)
		if err != nil {
			continue
		}
		os.WriteFile(destPath, []byte(data), 0644)
	}

	// Save metadata to DB
	desc := description
	skill := pg.SkillCreateParams{
		Name:        name,
		Slug:        slug,
		Description: &desc,
		OwnerID:     userID,
		Visibility:  "private",
		Version:     version,
		FilePath:    destDir,
		FileSize:    size,
		FileHash:    &fileHash,
	}

	id, err := h.skills.CreateSkillManaged(r.Context(), skill)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save skill: " + err.Error()})
		return
	}

	h.skills.BumpVersion()
	slog.Info("skill uploaded", "id", id, "slug", slug, "version", version, "size", header.Size)

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      id,
		"slug":    slug,
		"version": version,
		"name":    name,
	})
}

func (h *SkillsHandler) handleListAgentSkills(w http.ResponseWriter, r *http.Request) {
	agentIDStr := r.PathValue("agentID")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent ID"})
		return
	}

	skills, err := h.skills.ListWithGrantStatus(r.Context(), agentID)
	if err != nil {
		slog.Error("failed to list skills with grant status", "agent_id", agentID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list skills"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"skills": skills})
}
