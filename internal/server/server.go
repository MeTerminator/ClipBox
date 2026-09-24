package server

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math/big"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/MeTerminator/ClipBox/internal/config"
	"github.com/MeTerminator/ClipBox/internal/model"
	"github.com/MeTerminator/ClipBox/internal/share"
	"github.com/MeTerminator/ClipBox/internal/store"
	"github.com/MeTerminator/ClipBox/internal/upload"
	"github.com/gin-gonic/gin"
)

// Server adapts ClipBox application capabilities to HTTP and WebSocket APIs.
type Server struct {
	cfg     config.Config
	store   store.Store
	uploads *upload.Manager
	router  *gin.Engine
	www     fs.FS
	now     func() time.Time
	rooms   store.RoomStore
	sharing *share.Service
	hub     *share.Hub
}

// Dependencies contains the infrastructure required by the HTTP transport.
// Keeping construction explicit makes the composition root in main.go the
// only place that knows which concrete database and upload implementations are
// used. Tests can still provide small in-memory implementations.
type Dependencies struct {
	Clips   store.Store     // required
	Rooms   store.RoomStore // optional
	Uploads *upload.Manager // required
	WWW     fs.FS           // optional; when nil, WWWRoot is read from disk
}

type uploadInitRequest struct {
	Filename string `json:"filename" binding:"required"`
	Size     int64  `json:"size"`
	SHA1     string `json:"sha1" binding:"required"`
	Count    int    `json:"count"`
	Expire   int    `json:"expire"`
}

type uploadCompleteRequest struct {
	Filename string `json:"filename" binding:"required"`
	Count    int    `json:"count"`
	Expire   int    `json:"expire"`
}

// New builds a server from the legacy compact dependency list. New programs
// should prefer NewWithDependencies so optional capabilities are visible at
// the composition root.
func New(cfg config.Config, database store.Store, uploads *upload.Manager) *Server {
	dependencies := Dependencies{Clips: database, Uploads: uploads}
	if rooms, ok := database.(store.RoomStore); ok {
		dependencies.Rooms = rooms
	}
	return NewWithDependencies(cfg, dependencies)
}

// NewWithDependencies builds the HTTP transport and wires all routes. Room
// sharing is optional; when Rooms is nil its endpoints return 501.
func NewWithDependencies(cfg config.Config, dependencies Dependencies) *Server {
	server := &Server{
		cfg:     cfg,
		store:   dependencies.Clips,
		uploads: dependencies.Uploads,
		www:     dependencies.WWW,
		now:     func() time.Time { return time.Now().UTC() },
	}
	if dependencies.Rooms != nil {
		server.rooms = dependencies.Rooms
		server.sharing = share.NewService(dependencies.Rooms, cfg.MaxTextSize, func() time.Time { return server.now() })
		server.hub = share.NewHub()
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), server.cors())
	server.registerRoutes(router)
	server.registerFrontend(router)
	server.router = router
	return server
}

// Handler exposes the server as a standard net/http handler.
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) registerRoutes(router *gin.Engine) {
	api := router.Group("/api")
	api.GET("/config", s.getPublicConfig)
	clips := api.Group("/clip")
	clips.POST("/create", s.createClip)
	clips.GET("/:code/info", s.getClipInfo)
	clips.POST("/:code/resolve", s.resolveClip)
	clips.GET("/:code", s.getClip)

	uploads := clips.Group("/upload")
	uploads.POST("/init", s.initUpload)
	uploads.GET("/:uploadID", s.uploadStatus)
	uploads.PUT("/:uploadID/:chunk", s.uploadChunk)
	uploads.POST("/:uploadID/complete", s.completeUpload)

	api.GET("/file/:sha1/*filename", s.getFile)
	api.GET("/text/:sha1", s.getText)

	rooms := api.Group("/rooms")
	rooms.POST("", s.createRoom)
	rooms.POST("/:roomID/join", s.joinRoom)
	rooms.GET("/:roomID", s.getRoom)
	rooms.PATCH("/:roomID", s.renameRoom)
	rooms.PATCH("/:roomID/me", s.updateRoomNickname)
	rooms.PUT("/:roomID/password", s.updateRoomPassword)
	rooms.DELETE("/:roomID", s.deleteRoom)
	rooms.GET("/:roomID/messages", s.listRoomMessages)
	rooms.GET("/:roomID/ws", s.roomWebSocket)
	rooms.GET("/:roomID/messages/:messageID/file", s.downloadRoomFile)
}

func (s *Server) cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		allowOrigin := s.cfg.CORSAllowOrigin
		if allowOrigin == "" {
			allowOrigin = "*"
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/clip/upload/") {
			allowOrigin = "*"
		}
		if allowOrigin == "*" {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if c.GetHeader("Origin") == allowOrigin {
			c.Header("Access-Control-Allow-Origin", allowOrigin)
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
		c.Header("Access-Control-Max-Age", "600")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (s *Server) getPublicConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"site_url":            s.cfg.SiteURL,
		"upload_direct_first": s.cfg.UploadDirectFirst,
	})
}

func (s *Server) createClip(c *gin.Context) {
	count, expire, err := parseLimits(c.PostForm("count"), c.PostForm("expire"), 1000, 86400)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	content := strings.TrimSpace(c.PostForm("content"))
	if content == "" {
		jsonError(c, http.StatusBadRequest, "Missing content")
		return
	}
	isLink := strings.EqualFold(c.PostForm("link"), "yes")
	clip := &model.Clip{
		Content:       content,
		ClientIP:      s.realIP(c),
		AccessCount:   count,
		MaxCount:      count,
		ExpireSeconds: expire,
	}
	if isLink {
		if len(content) > s.cfg.MaxLinkLength || !validHTTPURL(content) {
			jsonError(c, http.StatusBadRequest, "Content must be a valid HTTP(S) URL")
			return
		}
		clip.ContentType = model.ContentLink
	} else {
		if int64(len([]byte(content))) > s.cfg.MaxTextSize {
			jsonError(c, http.StatusRequestEntityTooLarge, "Text content too large")
			return
		}
		clip.ContentType = model.ContentText
		clip.ContentHash = sha1String(content)
	}
	if err := s.createWithCode(c.Request.Context(), clip); err != nil {
		slog.Error("create clip", "error", err)
		jsonError(c, http.StatusInternalServerError, "Database error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": clip.Code})
}

func (s *Server) getClip(c *gin.Context) {
	code := c.Param("code")
	if len(code) != 5 || !allDigits(code) {
		jsonError(c, http.StatusNotFound, "Not found")
		return
	}
	clip, err := s.store.ConsumeByCode(c.Request.Context(), code, s.now())
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("consume clip", "code", code, "error", err)
		}
		jsonError(c, http.StatusNotFound, "Not found")
		return
	}
	switch clip.ContentType {
	case model.ContentLink:
		c.Redirect(http.StatusFound, clip.Content)
	case model.ContentText:
		hash := clip.ContentHash
		if hash == "" {
			hash = sha1String(clip.Content)
		}
		c.Redirect(http.StatusFound, "/api/text/"+hash)
	case model.ContentFile:
		if clip.File == nil || !upload.ValidSHA1(clip.File.SHA1) || clip.File.Filename == "" {
			jsonError(c, http.StatusNotFound, "Not found")
			return
		}
		c.Redirect(http.StatusFound, fileURL(clip.File.SHA1, clip.File.Filename))
	default:
		jsonError(c, http.StatusNotFound, "Not found")
	}
}

// getClipInfo returns current metadata without consuming an access.
func (s *Server) getClipInfo(c *gin.Context) {
	code := c.Param("code")
	if len(code) != 5 || !allDigits(code) {
		jsonError(c, http.StatusNotFound, "Not found")
		return
	}
	clip, err := s.store.FindByCode(c.Request.Context(), code)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("find clip info", "code", code, "error", err)
		}
		jsonError(c, http.StatusNotFound, "Not found")
		return
	}
	response := gin.H{
		"code":            clip.Code,
		"type":            clip.ContentType,
		"expires_at":      clip.CreatedAt.Add(time.Duration(clip.ExpireSeconds) * time.Second),
		"remaining_count": max(clip.AccessCount, 0),
		"max_count":       clip.MaxCount,
		"expired":         clip.Expired(s.now()),
	}
	if clip.ContentType == model.ContentFile && clip.File != nil {
		response["filename"] = clip.File.Filename
		response["size"] = clip.File.Size
	}
	c.JSON(http.StatusOK, response)
}

// resolveClip consumes one access and returns content for an interactive client.
func (s *Server) resolveClip(c *gin.Context) {
	code := c.Param("code")
	if len(code) != 5 || !allDigits(code) {
		jsonError(c, http.StatusNotFound, "Not found")
		return
	}
	clip, err := s.store.ConsumeByCode(c.Request.Context(), code, s.now())
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("resolve clip", "code", code, "error", err)
		}
		jsonError(c, http.StatusNotFound, "Not found")
		return
	}
	response := gin.H{
		"code":            clip.Code,
		"type":            clip.ContentType,
		"expires_at":      clip.CreatedAt.Add(time.Duration(clip.ExpireSeconds) * time.Second),
		"remaining_count": clip.AccessCount,
		"max_count":       clip.MaxCount,
	}
	switch clip.ContentType {
	case model.ContentLink, model.ContentText:
		response["content"] = clip.Content
	case model.ContentFile:
		if clip.File == nil || !upload.ValidSHA1(clip.File.SHA1) || clip.File.Filename == "" {
			jsonError(c, http.StatusNotFound, "Not found")
			return
		}
		response["filename"] = clip.File.Filename
		response["size"] = clip.File.Size
		response["download_url"] = fileURL(clip.File.SHA1, clip.File.Filename)
	default:
		jsonError(c, http.StatusNotFound, "Not found")
		return
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) getText(c *gin.Context) {
	hash := strings.ToLower(c.Param("sha1"))
	if !upload.ValidSHA1(hash) {
		jsonError(c, http.StatusNotFound, "Not found")
		return
	}
	clip, err := s.store.FindText(c.Request.Context(), hash, s.now())
	if err != nil {
		jsonError(c, http.StatusNotFound, "Not found")
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(clip.Content))
}

func (s *Server) getFile(c *gin.Context) {
	hash := strings.ToLower(c.Param("sha1"))
	filename := strings.TrimPrefix(c.Param("filename"), "/")
	if !upload.ValidSHA1(hash) || filename == "" || filename != filepath.Base(filename) {
		jsonError(c, http.StatusNotFound, "Not found")
		return
	}
	clip, err := s.store.FindFile(c.Request.Context(), hash, filename, s.now())
	if err != nil || clip.File == nil || clip.File.Path == "" {
		jsonError(c, http.StatusNotFound, "Not found")
		return
	}
	info, err := os.Stat(clip.File.Path)
	if err != nil || !info.Mode().IsRegular() {
		jsonError(c, http.StatusNotFound, "Not found")
		return
	}
	contentType := clip.File.MIMEType
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(filename))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(clip.File.Path)
}

func (s *Server) initUpload(c *gin.Context) {
	var request uploadInitRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid upload metadata")
		return
	}
	request.SHA1 = strings.ToLower(strings.TrimSpace(request.SHA1))
	request.Filename = safeFilename(request.Filename)
	count, expire, err := normalizedLimits(request.Count, request.Expire, 1000, 86400)
	if err != nil || request.Filename == "" || !upload.ValidSHA1(request.SHA1) || request.Size < 0 {
		jsonError(c, http.StatusBadRequest, "Invalid upload metadata")
		return
	}
	if request.Size > s.cfg.MaxUploadFileSize {
		jsonError(c, http.StatusRequestEntityTooLarge, "File too large")
		return
	}

	if reusable, findErr := s.store.FindReusableFile(c.Request.Context(), request.SHA1, request.Size, s.now()); findErr == nil {
		if reusable.File != nil {
			if info, statErr := os.Stat(reusable.File.Path); statErr == nil && info.Mode().IsRegular() && info.Size() == request.Size {
				clip := s.fileClip(request.Filename, reusable.File.Path, request.SHA1, request.Size, count, expire, c)
				if createErr := s.createWithCode(c.Request.Context(), clip); createErr != nil {
					jsonError(c, http.StatusInternalServerError, "Database error")
					return
				}
				c.JSON(http.StatusOK, gin.H{"instant_upload": true, "code": clip.Code, "url": fileURL(clip.File.SHA1, clip.File.Filename)})
				return
			}
		}
	}
	storedPath, found, findErr := s.uploads.FindStoredFile(request.SHA1, request.Size)
	if findErr != nil {
		slog.Error("find stored upload", "sha1", request.SHA1, "error", findErr)
	}
	if found {
		clip := s.fileClip(request.Filename, storedPath, request.SHA1, request.Size, count, expire, c)
		if createErr := s.createWithCode(c.Request.Context(), clip); createErr != nil {
			jsonError(c, http.StatusInternalServerError, "Database error")
			return
		}
		c.JSON(http.StatusOK, gin.H{"instant_upload": true, "code": clip.Code, "url": fileURL(clip.File.SHA1, clip.File.Filename)})
		return
	}

	status, err := s.uploads.Init(request.SHA1, request.Filename, request.Size, s.now())
	if err != nil {
		s.writeUploadError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"instant_upload":  false,
		"upload_id":       status.ID,
		"chunk_size":      status.ChunkSize,
		"total_chunks":    status.TotalChunks,
		"uploaded_chunks": status.UploadedChunks,
		"expires_at":      status.ExpiresAt,
		"workers":         s.cfg.UploadWorkers,
	})
}

func (s *Server) uploadStatus(c *gin.Context) {
	status, err := s.uploads.Status(strings.ToLower(c.Param("uploadID")), s.now())
	if err != nil {
		s.writeUploadError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (s *Server) uploadChunk(c *gin.Context) {
	index, err := upload.ParseChunkIndex(c.Param("chunk"))
	if err != nil {
		s.writeUploadError(c, err)
		return
	}
	err = s.uploads.WriteChunk(strings.ToLower(c.Param("uploadID")), index, c.Request.Body, s.now())
	if err != nil {
		s.writeUploadError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"chunk": index, "uploaded": true})
}

func (s *Server) completeUpload(c *gin.Context) {
	var request uploadCompleteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid upload metadata")
		return
	}
	request.Filename = safeFilename(request.Filename)
	count, expire, err := normalizedLimits(request.Count, request.Expire, 1000, 86400)
	if err != nil || request.Filename == "" {
		jsonError(c, http.StatusBadRequest, "Invalid upload metadata")
		return
	}
	hash := strings.ToLower(c.Param("uploadID"))
	path, size, err := s.uploads.Complete(hash, s.now())
	if err != nil {
		s.writeUploadError(c, err)
		return
	}
	clip := s.fileClip(request.Filename, path, hash, size, count, expire, c)
	if err := s.createWithCode(c.Request.Context(), clip); err != nil {
		slog.Error("save completed upload", "sha1", hash, "error", err)
		jsonError(c, http.StatusInternalServerError, "Database error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": clip.Code, "instant_upload": false, "url": fileURL(hash, request.Filename)})
}

func (s *Server) cleanupExpired(ctx context.Context, now time.Time) (int64, int, error) {
	paths, expired, err := s.store.CleanupExpired(ctx, now)
	if err != nil {
		return 0, 0, err
	}
	return expired, s.removeDataFiles(ctx, paths, now), nil
}

func (s *Server) removeDataFiles(ctx context.Context, paths []string, now time.Time) int {
	removed := 0
	for _, path := range paths {
		count, countErr := s.store.CountFileReferences(ctx, path, now)
		if countErr != nil || count != 0 || !s.pathInsideData(path) {
			continue
		}
		if removeErr := os.Remove(path); removeErr == nil {
			removed++
		}
	}
	return removed
}

// RunCleanup removes expired clips and unreferenced files for the lifetime of
// the server process. Upload sessions have their own shorter cleanup loop.
func (s *Server) RunCleanup(ctx context.Context) {
	run := func(now time.Time) {
		expired, removedFiles, err := s.cleanupExpired(ctx, now.UTC())
		if err != nil {
			slog.Error("cleanup expired clips", "error", err)
			return
		}
		if expired > 0 || removedFiles > 0 {
			slog.Info("cleaned expired clips", "clips", expired, "files", removedFiles)
		}
		if roomFiles, ok := s.store.(interface {
			CleanupRoomFileRetentions(context.Context, time.Time) ([]string, int64, error)
		}); ok {
			paths, files, err := roomFiles.CleanupRoomFileRetentions(ctx, now.UTC())
			if err != nil {
				slog.Error("cleanup expired room files", "error", err)
				return
			}
			removedRoomFiles := s.removeDataFiles(ctx, paths, now.UTC())
			if files > 0 || removedRoomFiles > 0 {
				slog.Info("cleaned expired room files", "metadata", files, "files", removedRoomFiles)
			}
		}
		s.cleanupEmptyRooms(ctx, now.UTC())
	}
	run(s.now())
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			run(now)
		}
	}
}

func (s *Server) cleanupEmptyRooms(ctx context.Context, now time.Time) {
	if s.rooms == nil || s.hub == nil {
		return
	}
	rooms, err := s.rooms.ListRooms(ctx)
	if err != nil {
		slog.Error("list empty share rooms", "error", err)
		return
	}
	for _, room := range rooms {
		// Allow the create response enough time to establish its first socket.
		if now.Sub(room.CreatedAt) < 30*time.Second || s.hub.OnlineCount(room.ID) != 0 {
			continue
		}
		if err := s.rooms.DeleteRoom(ctx, room.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
			slog.Error("delete empty share room", "room_id", room.PublicID, "error", err)
		}
	}
}

func (s *Server) fileClip(filename, path, hash string, size int64, count, expire int, c *gin.Context) *model.Clip {
	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return &model.Clip{
		ContentType: model.ContentFile,
		File: &model.File{
			Filename: filename,
			Path:     path,
			SHA1:     hash,
			Size:     size,
			MIMEType: mimeType,
		},
		ClientIP:      s.realIP(c),
		AccessCount:   count,
		MaxCount:      count,
		ExpireSeconds: expire,
	}
}

func (s *Server) createWithCode(ctx context.Context, clip *model.Clip) error {
	for attempts := 0; attempts < 30; attempts++ {
		code, err := randomCode()
		if err != nil {
			return err
		}
		clip.Code = code
		if err := s.store.Create(ctx, clip); err != nil {
			if errors.Is(err, store.ErrConflict) {
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("could not allocate a pickup code")
}

func (s *Server) realIP(c *gin.Context) string {
	if value := c.GetHeader(s.cfg.RealIPHeader); value != "" {
		if first, _, found := strings.Cut(value, ","); found {
			value = first
		}
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return c.ClientIP()
}

func (s *Server) writeUploadError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, upload.ErrInvalidUpload):
		jsonError(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, upload.ErrExpired):
		jsonError(c, http.StatusGone, err.Error())
	case errors.Is(err, upload.ErrIncomplete):
		jsonError(c, http.StatusConflict, err.Error())
	case errors.Is(err, upload.ErrHashMismatch):
		jsonError(c, http.StatusUnprocessableEntity, err.Error())
	default:
		slog.Error("upload operation", "error", err)
		jsonError(c, http.StatusInternalServerError, "Upload operation failed")
	}
}

func (s *Server) pathInsideData(path string) bool {
	dataRoot, err := filepath.Abs(s.cfg.DataDir)
	if err != nil {
		return false
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(dataRoot, target)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (s *Server) registerFrontend(router *gin.Engine) {
	if s.www != nil {
		router.NoRoute(s.serveEmbeddedFrontend)
		return
	}

	assets := filepath.Join(s.cfg.WWWRoot, "assets")
	if info, err := os.Stat(assets); err == nil && info.IsDir() {
		router.Static("/assets", assets)
	}
	router.NoRoute(func(c *gin.Context) {
		if s.serveAPINotFound(c) {
			return
		}
		requested := filepath.Join(s.cfg.WWWRoot, filepath.FromSlash(strings.TrimPrefix(c.Request.URL.Path, "/")))
		if info, err := os.Stat(requested); err == nil && info.Mode().IsRegular() && pathInside(s.cfg.WWWRoot, requested) {
			c.File(requested)
			return
		}
		index := filepath.Join(s.cfg.WWWRoot, "index.html")
		if _, err := os.Stat(index); err == nil {
			c.File(index)
			return
		}
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("ClipBox-API"))
	})
}

func (s *Server) serveEmbeddedFrontend(c *gin.Context) {
	if s.serveAPINotFound(c) {
		return
	}
	requestPath := strings.TrimPrefix(c.Request.URL.Path, "/")
	if requestPath != "" && fs.ValidPath(requestPath) {
		if info, err := fs.Stat(s.www, requestPath); err == nil && info.Mode().IsRegular() {
			if strings.HasPrefix(requestPath, "assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.FileServerFS(s.www).ServeHTTP(c.Writer, c.Request)
			return
		}
	}
	index, err := fs.ReadFile(s.www, "index.html")
	if err != nil {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("ClipBox-API"))
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", index)
}

func (s *Server) serveAPINotFound(c *gin.Context) bool {
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/api/") {
		jsonError(c, http.StatusNotFound, "Not found")
		return true
	}
	return false
}

func normalizedLimits(count, expire, defaultCount, defaultExpire int) (int, int, error) {
	if count == 0 {
		count = defaultCount
	}
	if expire == 0 {
		expire = defaultExpire
	}
	const maxDatabaseInt = int(^uint32(0) >> 1)
	if count < 1 || expire < 1 || count > maxDatabaseInt || expire > maxDatabaseInt {
		return 0, 0, fmt.Errorf("count and expire must be between 1 and %d", maxDatabaseInt)
	}
	return count, expire, nil
}

func parseLimits(rawCount, rawExpire string, defaultCount, defaultExpire int) (int, int, error) {
	count, expire := defaultCount, defaultExpire
	var err error
	if rawCount != "" {
		count, err = strconv.Atoi(rawCount)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid count")
		}
	}
	if rawExpire != "" {
		expire, err = strconv.Atoi(rawExpire)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid expire")
		}
	}
	return normalizedLimits(count, expire, defaultCount, defaultExpire)
}

func validHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func safeFilename(value string) string {
	value = filepath.Base(strings.TrimSpace(strings.ReplaceAll(value, "\\", "/")))
	if value == "." || value == ".." {
		return ""
	}
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\\|?*`, r) {
			return '_'
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	if len([]byte(value)) > 255 {
		return ""
	}
	return value
}

func randomCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(90000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%05d", value.Int64()+10000), nil
}

func sha1String(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func fileURL(hash, filename string) string {
	return "/api/file/" + hash + "/" + url.PathEscape(filename)
}

func allDigits(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func jsonError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}

func pathInside(root, target string) bool {
	rootAbs, rootErr := filepath.Abs(root)
	targetAbs, targetErr := filepath.Abs(target)
	if rootErr != nil || targetErr != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbs, targetAbs)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
