package controller

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gameauth "github.com/akorwash/QuizBattle/auth"
	avatardomain "github.com/akorwash/QuizBattle/domain/avatar"
	"github.com/akorwash/QuizBattle/repository"
	"github.com/akorwash/QuizBattle/resources"
)

type avatarServiceStub struct {
	uploadUser int64
	upload     []byte
	metadata   *resources.UserAvatar
	content    *resources.UserAvatarContent
	getUser    int64
	deleteUser int64
	err        error
}

func (stub *avatarServiceStub) SaveOrReplace(_ context.Context, userID int64, upload []byte) (*resources.UserAvatar, error) {
	stub.uploadUser = userID
	stub.upload = append([]byte(nil), upload...)
	return stub.metadata, stub.err
}

func (stub *avatarServiceStub) Get(_ context.Context, userID int64) (*resources.UserAvatarContent, error) {
	stub.getUser = userID
	return stub.content, stub.err
}

func (stub *avatarServiceStub) Delete(_ context.Context, userID int64) error {
	stub.deleteUser = userID
	return stub.err
}

func TestAvatarUploadUsesAuthenticatedIdentityAndMultipartField(t *testing.T) {
	stub := &avatarServiceStub{metadata: &resources.UserAvatar{
		UserID: 42, URL: "/api/v1/user/avatar/42", ETag: `"abc"`, ContentType: "image/jpeg",
	}}
	request := avatarUploadRequest(t, AvatarFileField, []byte("untrusted bytes"), nil)
	request = withAvatarIdentity(request, 42)
	response := httptest.NewRecorder()
	new(AvatarController).Upload(stub).ServeHTTP(response, request)
	if response.Code != http.StatusOK || stub.uploadUser != 42 || !bytes.Equal(stub.upload, []byte("untrusted bytes")) {
		t.Fatalf("upload contract failed: status=%d user=%d data=%q", response.Code, stub.uploadUser, stub.upload)
	}
	if response.Header().Get("ETag") != `"abc"` || response.Header().Get("Location") != "/api/v1/user/avatar/42" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("upload headers are incomplete: %#v", response.Header())
	}
}

func TestAvatarUploadRejectsUnauthenticatedOrMalformedBodies(t *testing.T) {
	controller := new(AvatarController)
	stub := &avatarServiceStub{}
	unauthenticated := avatarUploadRequest(t, AvatarFileField, []byte("image"), nil)
	response := httptest.NewRecorder()
	controller.Upload(stub).ServeHTTP(response, unauthenticated)
	if response.Code != http.StatusUnauthorized || stub.uploadUser != 0 {
		t.Fatalf("unauthenticated upload reached service: status=%d user=%d", response.Code, stub.uploadUser)
	}

	wrongField := withAvatarIdentity(avatarUploadRequest(t, "file", []byte("image"), nil), 42)
	response = httptest.NewRecorder()
	controller.Upload(stub).ServeHTTP(response, wrongField)
	if response.Code != http.StatusBadRequest || stub.uploadUser != 0 {
		t.Fatalf("wrong multipart field returned %d", response.Code)
	}

	oversized := withAvatarIdentity(avatarUploadRequest(t, AvatarFileField, make([]byte, avatardomain.MaxUploadBytes+1), nil), 42)
	response = httptest.NewRecorder()
	controller.Upload(stub).ServeHTTP(response, oversized)
	if response.Code != http.StatusRequestEntityTooLarge || stub.uploadUser != 0 {
		t.Fatalf("oversized upload returned %d", response.Code)
	}

	unknownValue := withAvatarIdentity(avatarUploadRequest(t, AvatarFileField, []byte("image"), map[string]string{"userId": "99"}), 42)
	response = httptest.NewRecorder()
	controller.Upload(stub).ServeHTTP(response, unknownValue)
	if response.Code != http.StatusBadRequest || stub.uploadUser != 0 {
		t.Fatalf("client-supplied fields returned %d", response.Code)
	}
}

func TestAvatarGetUsesPrivateETagRevalidation(t *testing.T) {
	updatedAt := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	stub := &avatarServiceStub{content: &resources.UserAvatarContent{
		UserID: 99, Data: []byte("canonical-jpeg"), ETag: "sha256-value", ContentType: "image/jpeg", UpdatedAt: updatedAt,
	}}
	controller := new(AvatarController)
	request := withAvatarIdentity(httptest.NewRequest(http.MethodGet, "/api/v1/user/avatar/99", nil), 42)
	request.SetPathValue("id", "99")
	response := httptest.NewRecorder()
	controller.Get(stub).ServeHTTP(response, request)
	if response.Code != http.StatusOK || stub.getUser != 99 || response.Body.String() != "canonical-jpeg" {
		t.Fatalf("avatar read failed: status=%d target=%d body=%q", response.Code, stub.getUser, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-cache" || response.Header().Get("ETag") != `"sha256-value"` || response.Header().Get("Content-Type") != "image/jpeg" || response.Header().Get("Last-Modified") == "" {
		t.Fatalf("avatar cache headers are incomplete: %#v", response.Header())
	}

	request = withAvatarIdentity(httptest.NewRequest(http.MethodGet, "/api/v1/user/avatar/99", nil), 42)
	request.SetPathValue("id", "99")
	request.Header.Set("If-None-Match", `W/"sha256-value"`)
	response = httptest.NewRecorder()
	controller.Get(stub).ServeHTTP(response, request)
	if response.Code != http.StatusNotModified || response.Body.Len() != 0 || response.Header().Get("ETag") != `"sha256-value"` {
		t.Fatalf("conditional GET failed: status=%d body=%q headers=%#v", response.Code, response.Body.String(), response.Header())
	}
}

func TestAvatarGetRequiresAuthenticationAndValidTarget(t *testing.T) {
	stub := &avatarServiceStub{content: &resources.UserAvatarContent{}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/user/avatar/99", nil)
	request.SetPathValue("id", "99")
	response := httptest.NewRecorder()
	new(AvatarController).Get(stub).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || stub.getUser != 0 {
		t.Fatalf("unauthenticated read: status=%d target=%d", response.Code, stub.getUser)
	}

	request = withAvatarIdentity(httptest.NewRequest(http.MethodGet, "/api/v1/user/avatar/not-an-id", nil), 42)
	request.SetPathValue("id", "not-an-id")
	response = httptest.NewRecorder()
	new(AvatarController).Get(stub).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || stub.getUser != 0 {
		t.Fatalf("invalid target returned %d", response.Code)
	}
}

func TestAvatarDeleteUsesOnlyAuthenticatedIdentityAndIsIdempotent(t *testing.T) {
	stub := &avatarServiceStub{}
	request := withAvatarIdentity(httptest.NewRequest(http.MethodDelete, "/api/v1/user/avatar", nil), 42)
	response := httptest.NewRecorder()
	new(AvatarController).Delete(stub).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || stub.deleteUser != 42 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("delete failed: status=%d user=%d", response.Code, stub.deleteUser)
	}
}

func TestAvatarErrorsMapToSafeHTTPResponses(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{avatardomain.ErrImageTooLarge, http.StatusRequestEntityTooLarge},
		{avatardomain.ErrUnsupportedFormat, http.StatusUnsupportedMediaType},
		{avatardomain.ErrInvalidDimensions, http.StatusBadRequest},
		{repository.ErrNotFound, http.StatusNotFound},
		{errors.New("database unavailable"), http.StatusInternalServerError},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		respondAvatarError(response, test.err)
		if response.Code != test.status || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("error %v mapped to %d, want %d", test.err, response.Code, test.status)
		}
	}
}

func avatarUploadRequest(t *testing.T, field string, data []byte, values map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, "avatar.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	for key, value := range values {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/v1/user/avatar", bytes.NewReader(body.Bytes()))
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func withAvatarIdentity(request *http.Request, userID int64) *http.Request {
	return request.WithContext(gameauth.WithIdentity(request.Context(), gameauth.Identity{UserID: userID, Username: "player"}))
}
