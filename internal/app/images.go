package app

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	sqlcdb "github.com/edwlarkey/orthodoxpilgrimage/internal/db/sqlc"
)

// ImageMetadata contains information about a processed image
type ImageMetadata struct {
	EntityType    string
	EntityID      int64
	RelicChurchID int64
	RelicSaintID  int64
	AltText       string
	Source        string
	IsPrimary     bool
}

// ProcessAndUploadImage handles the full pipeline: optimize -> upload optimized -> upload original -> save to DB
func (a *Application) ProcessAndUploadImage(ctx context.Context, file io.Reader, filename string, meta ImageMetadata) error {
	if a.S3Client == nil {
		return fmt.Errorf("S3 client not initialized")
	}

	// Create temp directory for processing
	tempDir, err := os.MkdirTemp("", "imgproc-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	originalExt := filepath.Ext(filename)
	baseFilename := strings.TrimSuffix(filename, originalExt)
	originalPath := filepath.Join(tempDir, "original"+originalExt)
	optimizedPath := filepath.Join(tempDir, "optimized.webp")

	// Save original to disk for processing
	out, err := os.Create(originalPath) //nolint:gosec // G703: path is from os.MkdirTemp, not user input
	if err != nil {
		return fmt.Errorf("failed to create original temp file: %w", err)
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		return fmt.Errorf("failed to save original temp file: %w", err)
	}
	out.Close()

	// 1. Process with ImageMagick
	magickCmd := "magick"
	if _, err := exec.LookPath(magickCmd); err != nil {
		magickCmd = "convert"
		if _, err := exec.LookPath(magickCmd); err != nil {
			return fmt.Errorf("neither 'magick' nor 'convert' binary found")
		}
	}

	args := []string{
		originalPath,
		"-auto-orient",
		"-resize", "800x>",
		"-strip",
		"-quality", "80",
		"-define", "webp:method=6",
		optimizedPath,
	}

	//nolint:gosec
	cmd := exec.CommandContext(ctx, magickCmd, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("imagemagick failed: %w (stderr: %s)", err, stderr.String())
	}

	// 2. Upload to S3
	var optimizedS3Key, originalS3Key string
	switch meta.EntityType {
	case "church":
		church, err := a.DB.GetChurch(ctx, meta.EntityID)
		if err != nil {
			return fmt.Errorf("failed to get church for slug: %w", err)
		}
		optimizedS3Key = fmt.Sprintf("churches/%s/optimized/%s.webp", church.Slug, baseFilename)
		originalS3Key = fmt.Sprintf("churches/%s/original/%s", church.Slug, filename)
	case "saint":
		saint, err := a.DB.GetSaint(ctx, meta.EntityID)
		if err != nil {
			return fmt.Errorf("failed to get saint for slug: %w", err)
		}
		optimizedS3Key = fmt.Sprintf("saints/%s/optimized/%s.webp", saint.Slug, baseFilename)
		originalS3Key = fmt.Sprintf("saints/%s/original/%s", saint.Slug, filename)
	case "relic":
		church, err := a.DB.GetChurch(ctx, meta.RelicChurchID)
		if err != nil {
			return fmt.Errorf("failed to get church for relic slug: %w", err)
		}
		saint, err := a.DB.GetSaint(ctx, meta.RelicSaintID)
		if err != nil {
			return fmt.Errorf("failed to get saint for relic slug: %w", err)
		}
		optimizedS3Key = fmt.Sprintf("relics/%s/%s/optimized/%s.webp", church.Slug, saint.Slug, baseFilename)
		originalS3Key = fmt.Sprintf("relics/%s/%s/original/%s", church.Slug, saint.Slug, filename)
	default:
		return fmt.Errorf("unsupported entity type for S3 key: %s", meta.EntityType)
	}

	// Upload Optimized
	if err := a.uploadToS3(ctx, optimizedPath, optimizedS3Key, "image/webp"); err != nil {
		return fmt.Errorf("failed to upload optimized image: %w", err)
	}

	// Upload Original
	f, _ := os.Open(originalPath) //nolint:gosec // G703: originalPath is generated securely in a temporary directory via os.CreateTemp.
	buffer := make([]byte, 512)
	_, _ = f.Read(buffer)
	f.Close()
	contentType := http.DetectContentType(buffer)
	if err := a.uploadToS3(ctx, originalPath, originalS3Key, contentType); err != nil {
		return fmt.Errorf("failed to upload original image: %w", err)
	}

	// 3. Save to Database
	imageURL := fmt.Sprintf("https://images.orthodoxpilgrimage.com/%s", optimizedS3Key)

	params := sqlcdb.CreateImageParams{
		Url:       imageURL,
		AltText:   sql.NullString{String: meta.AltText, Valid: meta.AltText != ""},
		Source:    sql.NullString{String: meta.Source, Valid: meta.Source != ""},
		IsPrimary: sql.NullBool{Bool: meta.IsPrimary, Valid: true},
	}

	switch meta.EntityType {
	case "church":
		params.ChurchID = sql.NullInt64{Int64: meta.EntityID, Valid: true}
		if meta.IsPrimary {
			_ = a.DB.UnsetPrimaryImageForChurch(ctx, params.ChurchID)
		}
	case "saint":
		params.SaintID = sql.NullInt64{Int64: meta.EntityID, Valid: true}
		if meta.IsPrimary {
			_ = a.DB.UnsetPrimaryImageForSaint(ctx, params.SaintID)
		}
	case "relic":
		params.RelicChurchID = sql.NullInt64{Int64: meta.RelicChurchID, Valid: true}
		params.RelicSaintID = sql.NullInt64{Int64: meta.RelicSaintID, Valid: true}
	default:
		return fmt.Errorf("unsupported entity type: %s", meta.EntityType)
	}

	if err := a.DB.CreateImage(ctx, params); err != nil {
		return fmt.Errorf("failed to save image to database: %w", err)
	}

	return nil
}

func (a *Application) uploadToS3(ctx context.Context, localPath, s3Key, contentType string) error {
	file, err := os.Open(localPath) //nolint:gosec // G703: localPath is generated securely in a temporary directory via os.CreateTemp or derived securely.
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = a.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.S3Bucket),
		Key:         aws.String(s3Key),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	return err
}

func (a *Application) adminImageUploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Max 32MB total upload
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		slog.Error("Failed to parse multipart form", "error", err)
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	entityType := r.FormValue("entity_type")
	entityIDStr := r.FormValue("entity_id")
	entityID, _ := strconv.ParseInt(entityIDStr, 10, 64)

	relicChurchID, _ := strconv.ParseInt(r.FormValue("relic_church_id"), 10, 64)
	relicSaintID, _ := strconv.ParseInt(r.FormValue("relic_saint_id"), 10, 64)

	files := r.MultipartForm.File["images"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			slog.Error("Failed to open uploaded file", "error", err)
			continue
		}

		err = a.ProcessAndUploadImage(r.Context(), file, fileHeader.Filename, ImageMetadata{
			EntityType:    entityType,
			EntityID:      entityID,
			RelicChurchID: relicChurchID,
			RelicSaintID:  relicSaintID,
			IsPrimary:     false,
		})
		file.Close()

		if err != nil {
			slog.Error("Failed to process image", "error", err)
			http.Error(w, "Failed to process image: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Trigger HTMX refresh of the image gallery
	w.Header().Set("HX-Trigger", "imageGalleryUpdated")
	w.WriteHeader(http.StatusNoContent)
}

func (a *Application) adminImageGalleryHandler(w http.ResponseWriter, r *http.Request) {
	entityType := r.URL.Query().Get("entity_type")
	entityIDStr := r.URL.Query().Get("entity_id")
	entityID, _ := strconv.ParseInt(entityIDStr, 10, 64)

	var images []sqlcdb.Image
	var err error

	switch entityType {
	case "church":
		images, err = a.DB.ListImagesForChurch(r.Context(), sql.NullInt64{Int64: entityID, Valid: true})
	case "saint":
		images, err = a.DB.ListImagesForSaint(r.Context(), sql.NullInt64{Int64: entityID, Valid: true})
	case "relic":
		relicChurchID, _ := strconv.ParseInt(r.URL.Query().Get("relic_church_id"), 10, 64)
		relicSaintID, _ := strconv.ParseInt(r.URL.Query().Get("relic_saint_id"), 10, 64)
		images, err = a.DB.ListImagesForRelic(r.Context(), sqlcdb.ListImagesForRelicParams{
			RelicChurchID: sql.NullInt64{Int64: relicChurchID, Valid: true},
			RelicSaintID:  sql.NullInt64{Int64: relicSaintID, Valid: true},
		})
		// For relic display, we might want to override entityID for template mapping
		entityID = relicChurchID // Just a placeholder
	default:
		http.Error(w, "Invalid entity type", http.StatusBadRequest)
		return
	}

	if err != nil {
		slog.Error("Failed to list images", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Images":     images,
		"EntityType": entityType,
		"EntityID":   entityID,
	}

	ts, err := a.Templates.Get("admin-image-gallery")
	if err != nil {
		slog.Error("Template not found", "error", err)
		// Minimal fallback for HTMX
		for _, img := range images {
			fmt.Fprintf(w, "<div class='image-item'><img src='%s' width='100'><button hx-delete='/admin/images/delete?id=%d' hx-target='closest .image-item' hx-swap='outerHTML'>Delete</button></div>", img.Url, img.ID)
		}
		return
	}

	err = ts.ExecuteTemplate(w, "admin-image-gallery", data)
	if err != nil {
		slog.Error("Error rendering image gallery", "error", err)
	}
}

func (a *Application) adminImageDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		idStr = r.FormValue("id")
	}
	id, _ := strconv.ParseInt(idStr, 10, 64)

	image, err := a.DB.GetImage(r.Context(), id)
	if err != nil {
		slog.Error("Failed to find image for deletion", "id", id, "error", err)
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	// 1. Delete from S3
	if a.S3Client != nil {
		key := strings.TrimPrefix(image.Url, "https://images.orthodoxpilgrimage.com/")

		_, err = a.S3Client.DeleteObject(r.Context(), &s3.DeleteObjectInput{
			Bucket: aws.String(a.S3Bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			slog.Error("Failed to delete from S3", "key", key, "error", err)
		}
	}

	// 2. Delete from DB
	err = a.DB.DeleteImage(r.Context(), id)
	if err != nil {
		slog.Error("Failed to delete image from DB", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	a.logAudit(r.Context(), "DELETE", "image", id, nil)

	if r.Header.Get("HX-Request") != "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *Application) adminImageUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	altText := r.FormValue("alt_text")
	isPrimary := r.FormValue("is_primary") == "true"

	image, err := a.DB.GetImage(r.Context(), id)
	if err != nil {
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	if isPrimary {
		if image.ChurchID.Valid {
			_ = a.DB.UnsetPrimaryImageForChurch(r.Context(), image.ChurchID)
		} else if image.SaintID.Valid {
			_ = a.DB.UnsetPrimaryImageForSaint(r.Context(), image.SaintID)
		}
	}

	err = a.DB.UpdateImage(r.Context(), sqlcdb.UpdateImageParams{
		ID:        id,
		AltText:   sql.NullString{String: altText, Valid: true},
		IsPrimary: sql.NullBool{Bool: isPrimary, Valid: true},
		SortOrder: image.SortOrder,
	})

	if err != nil {
		slog.Error("Failed to update image", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", "imageGalleryUpdated")
	w.WriteHeader(http.StatusNoContent)
}
