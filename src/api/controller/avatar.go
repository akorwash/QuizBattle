package controller

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	avatardomain "github.com/akorwash/QuizBattle/domain/avatar"
	"github.com/akorwash/QuizBattle/service"
)

const (
	AvatarFileField         = "avatar"
	maxAvatarMultipartBytes = avatardomain.MaxUploadBytes + 64<<10
)

type AvatarController struct{}

// Upload creates or replaces the authenticated user's avatar. The multipart
// body must contain exactly one file field named "avatar".
func (controller *AvatarController) Upload(avatarService service.AvatarServices) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		upload, err := readAvatarUpload(w, r)
		if err != nil {
			respondAvatarError(w, err)
			return
		}
		result, err := avatarService.SaveOrReplace(r.Context(), identity.UserID, upload)
		if err != nil {
			respondAvatarError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("ETag", result.ETag)
		w.Header().Set("Location", result.URL)
		responseHandler.RespondWithJSON(w, http.StatusOK, result)
	}
}

// Get serves any user's avatar to an authenticated caller. Avatar responses
// are private and always revalidated so a replacement becomes visible without
// allowing shared caches to retain profile images.
func (controller *AvatarController) Get(avatarService service.AvatarServices) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := Identity(r); err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		userID, err := avatarUserID(r.PathValue("id"))
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		content, err := avatarService.Get(r.Context(), userID)
		if err != nil {
			w.Header().Set("Cache-Control", "private, no-cache")
			respondAvatarError(w, err)
			return
		}

		etag := quotedAvatarETag(content.ETag)
		w.Header().Set("Cache-Control", "private, no-cache")
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", content.UpdatedAt.UTC().Format(http.TimeFormat))
		if requestETagMatches(r.Header.Get("If-None-Match"), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", content.ContentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(content.Data)))
		w.WriteHeader(http.StatusOK)
		// #nosec G705 -- bytes are a server-generated JPEG with a fixed image MIME type.
		_, _ = w.Write(content.Data)
	}
}

// Delete removes the authenticated user's current avatar. It is idempotent.
func (controller *AvatarController) Delete(avatarService service.AvatarServices) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := Identity(r)
		if err != nil {
			responseHandler.RespondWithError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if err := avatarService.Delete(r.Context(), identity.UserID); err != nil {
			respondAvatarError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	}
}

func readAvatarUpload(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	mediaType, parameters, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") || parameters["boundary"] == "" {
		return nil, avatardomain.ErrUnsupportedFormat
	}
	if r.ContentLength > maxAvatarMultipartBytes {
		return nil, avatardomain.ErrImageTooLarge
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarMultipartBytes)
	// #nosec G120 -- MaxBytesReader above enforces a stricter bound on the complete body.
	if err := r.ParseMultipartForm(avatardomain.MaxUploadBytes); err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			return nil, avatardomain.ErrImageTooLarge
		}
		return nil, fmt.Errorf("%w: malformed multipart body", avatardomain.ErrInvalidImage)
	}
	if r.MultipartForm == nil {
		return nil, fmt.Errorf("%w: malformed multipart body", avatardomain.ErrInvalidImage)
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File[AvatarFileField]
	if len(files) != 1 || len(r.MultipartForm.File) != 1 || len(r.MultipartForm.Value) != 0 {
		return nil, fmt.Errorf("%w: exactly one %q file is required", avatardomain.ErrInvalidImage, AvatarFileField)
	}
	fileHeader := files[0]
	if fileHeader.Size <= 0 {
		return nil, avatardomain.ErrInvalidImage
	}
	if fileHeader.Size > avatardomain.MaxUploadBytes {
		return nil, avatardomain.ErrImageTooLarge
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: open upload", avatardomain.ErrInvalidImage)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, avatardomain.MaxUploadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read upload", avatardomain.ErrInvalidImage)
	}
	if len(data) > avatardomain.MaxUploadBytes {
		return nil, avatardomain.ErrImageTooLarge
	}
	if len(data) == 0 {
		return nil, avatardomain.ErrInvalidImage
	}
	return data, nil
}

func avatarUserID(value string) (int64, error) {
	userID, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || userID <= 0 {
		return 0, fmt.Errorf("invalid user ID")
	}
	return userID, nil
}

func quotedAvatarETag(value string) string {
	return `"` + value + `"`
}

func requestETagMatches(header, current string) bool {
	for candidate := range strings.SplitSeq(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" {
			return true
		}
		candidate = strings.TrimPrefix(candidate, "W/")
		if candidate == current {
			return true
		}
	}
	return false
}

func respondAvatarError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, avatardomain.ErrImageTooLarge):
		responseHandler.RespondWithError(w, http.StatusRequestEntityTooLarge, "avatar must not exceed 2 MiB")
	case errors.Is(err, avatardomain.ErrUnsupportedFormat):
		responseHandler.RespondWithError(w, http.StatusUnsupportedMediaType, "avatar must be a JPEG or PNG image")
	case errors.Is(err, avatardomain.ErrInvalidImage), errors.Is(err, avatardomain.ErrInvalidDimensions):
		responseHandler.RespondWithError(w, http.StatusBadRequest, err.Error())
	default:
		respondServiceError(w, err)
	}
}
