package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"api-hamy/internal/domain"
	adminservice "api-hamy/internal/service/admin"
	albumservice "api-hamy/internal/service/album"
	authservice "api-hamy/internal/service/auth"
	photoservice "api-hamy/internal/service/photo"
	userservice "api-hamy/internal/service/user"
	"api-hamy/internal/testutil"
	"api-hamy/internal/token"
	apihttp "api-hamy/internal/transport/http"
	"api-hamy/internal/transport/http/handler"
)

type fakePinger struct{ err error }

func (p *fakePinger) Ping(context.Context) error { return p.err }

type fakeProbe struct{ err error }

func (p *fakeProbe) EnsureBucket(context.Context) error { return p.err }

type harness struct {
	router *gin.Engine
	users  *testutil.FakeUserRepo
	albums *testutil.FakeAlbumRepo
	photos *testutil.FakePhotoRepo
	blobs  *testutil.FakeBlobStore
	pinger *fakePinger
	probe  *fakeProbe
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	users := testutil.NewFakeUserRepo()
	albums := testutil.NewFakeAlbumRepo(users)
	photos := testutil.NewFakePhotoRepo(albums)
	blobs := testutil.NewFakeBlobStore()
	albums.PhotoLookup = func(photoID string) (string, bool, bool) {
		p, ok := photos.Get(photoID)
		if !ok {
			return "", false, false
		}
		return p.AlbumID, p.Status == domain.PhotoReady, true
	}

	userSvc := userservice.New(users, logger)
	manager, err := token.NewManager("test-secret-with-at-least-32-characters", "test", time.Hour)
	if err != nil {
		t.Fatalf("token.NewManager() error = %v", err)
	}
	authSvc := authservice.New(userSvc, manager, logger)
	adminSvc := adminservice.New(userSvc, logger)
	albumSvc := albumservice.New(albums, users, logger)
	photoSvc := photoservice.New(photos, albums, blobs, "test-bucket", 15*time.Minute, logger)

	pinger := &fakePinger{}
	probe := &fakeProbe{}

	router := apihttp.NewRouter(
		handler.NewHealthHandler(pinger, probe),
		handler.NewAuthHandler(authSvc),
		authSvc,
		handler.NewAlbumHandler(albumSvc),
		handler.NewPhotoHandler(photoSvc),
		handler.NewAdminHandler(adminSvc),
		[]string{"http://app.test"},
	)

	return &harness{
		router: router,
		users:  users,
		albums: albums,
		photos: photos,
		blobs:  blobs,
		pinger: pinger,
		probe:  probe,
	}
}

func (h *harness) do(method, path, token, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), target); err != nil {
		t.Fatalf("invalid JSON %q: %v", w.Body.String(), err)
	}
}

type envelope[T any] struct {
	Data T `json:"data"`
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func expectStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, want, w.Body.String())
	}
}

func expectErrorCode(t *testing.T, w *httptest.ResponseRecorder, want int, code string) {
	t.Helper()
	expectStatus(t, w, want)

	var body errorEnvelope
	decode(t, w, &body)
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q (body: %s)", body.Error.Code, code, w.Body.String())
	}
}

type sessionBody struct {
	AccessToken string      `json:"access_token"`
	TokenType   string      `json:"token_type"`
	User        domain.User `json:"user"`
}

// register creates an account through the public endpoint and returns the user
// ID and access token.
func (h *harness) register(t *testing.T, name, email, password string) (string, string) {
	t.Helper()

	body := `{"name":"` + name + `","email":"` + email + `","password":"` + password + `"}`
	w := h.do(http.MethodPost, "/api/v1/auth/register", "", body)
	expectStatus(t, w, http.StatusCreated)

	var session envelope[sessionBody]
	decode(t, w, &session)
	if session.Data.AccessToken == "" || session.Data.User.ID == "" {
		t.Fatalf("register session = %+v, want token and user", session.Data)
	}
	return session.Data.User.ID, session.Data.AccessToken
}

func (h *harness) promote(t *testing.T, userID string) {
	t.Helper()

	u, err := h.users.GetByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	u.IsAdmin = true
	if err := h.users.Update(context.Background(), u); err != nil {
		t.Fatalf("promote() error = %v", err)
	}
}

func TestHealthAndReadiness(t *testing.T) {
	h := newHarness(t)

	expectStatus(t, h.do(http.MethodGet, "/healthz", "", ""), http.StatusOK)
	expectStatus(t, h.do(http.MethodGet, "/readyz", "", ""), http.StatusOK)

	h.pinger.err = errors.New("db down")
	w := h.do(http.MethodGet, "/readyz", "", "")
	expectStatus(t, w, http.StatusServiceUnavailable)
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	decode(t, w, &body)
	if body.Status != "unavailable" || body.Checks["database"] != "error" {
		t.Fatalf("readyz body = %+v, want database error", body)
	}

	// Storage alone also fails readiness.
	h.pinger.err = nil
	h.probe.err = errors.New("bucket down")
	expectStatus(t, h.do(http.MethodGet, "/readyz", "", ""), http.StatusServiceUnavailable)
}

func TestAuthEndpoints(t *testing.T) {
	h := newHarness(t)

	w := h.do(http.MethodPost, "/api/v1/auth/register", "", `{"name":"Ana","email":"ANA@Example.com","password":"supersecret"}`)
	expectStatus(t, w, http.StatusCreated)

	var created envelope[sessionBody]
	decode(t, w, &created)
	if created.Data.AccessToken == "" || created.Data.TokenType != "Bearer" {
		t.Fatalf("register = %+v, want a bearer token", created.Data)
	}
	if created.Data.User.Email != "ana@example.com" || created.Data.User.IsAdmin {
		t.Fatalf("register user = %+v, want normalized non-admin", created.Data.User)
	}
	if strings.Contains(w.Body.String(), "password") {
		t.Fatalf("register response leaks a password field: %s", w.Body.String())
	}

	expectErrorCode(t, h.do(http.MethodPost, "/api/v1/auth/register", "", `{"name":"Ana","email":"ana@example.com","password":"supersecret"}`), http.StatusConflict, "conflict")
	expectErrorCode(t, h.do(http.MethodPost, "/api/v1/auth/register", "", `{"name":"A","email":"nope","password":"supersecret"}`), http.StatusBadRequest, "invalid_request")
	expectErrorCode(t, h.do(http.MethodPost, "/api/v1/auth/register", "", `{"name":"Bea","email":"bea@example.com","password":"short"}`), http.StatusBadRequest, "invalid_request")

	// Login accepts the normalized email.
	w = h.do(http.MethodPost, "/api/v1/auth/login", "", `{"email":"ANA@example.com","password":"supersecret"}`)
	expectStatus(t, w, http.StatusOK)
	var loggedIn envelope[sessionBody]
	decode(t, w, &loggedIn)
	if loggedIn.Data.AccessToken == "" {
		t.Fatal("login returned no token")
	}

	expectErrorCode(t, h.do(http.MethodPost, "/api/v1/auth/login", "", `{"email":"ana@example.com","password":"wrong"}`), http.StatusUnauthorized, "unauthorized")
	expectErrorCode(t, h.do(http.MethodPost, "/api/v1/auth/login", "", `{"email":"nobody@example.com","password":"supersecret"}`), http.StatusUnauthorized, "unauthorized")

	w = h.do(http.MethodGet, "/api/v1/auth/me", loggedIn.Data.AccessToken, "")
	expectStatus(t, w, http.StatusOK)
	var me envelope[domain.User]
	decode(t, w, &me)
	if me.Data.ID != created.Data.User.ID || me.Data.Email != "ana@example.com" {
		t.Fatalf("me = %+v, want the registered user", me.Data)
	}
}

func TestAuthMiddleware(t *testing.T) {
	h := newHarness(t)

	expectErrorCode(t, h.do(http.MethodGet, "/api/v1/albums", "", ""), http.StatusUnauthorized, "unauthorized")
	expectErrorCode(t, h.do(http.MethodGet, "/api/v1/albums", "not-a-token", ""), http.StatusUnauthorized, "unauthorized")

	// A malformed scheme is rejected too.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Basic abc")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	expectStatus(t, w, http.StatusUnauthorized)
}

func TestFullAlbumFlow(t *testing.T) {
	h := newHarness(t)

	_, ana := h.register(t, "Ana", "ana@example.com", "supersecret")
	luisID, luis := h.register(t, "Luis", "luis@example.com", "supersecret")
	_, marta := h.register(t, "Marta", "marta@example.com", "supersecret")

	// Create the album.
	w := h.do(http.MethodPost, "/api/v1/albums", ana, `{"title":"Viaje","description":"Playa"}`)
	expectStatus(t, w, http.StatusCreated)
	var albumBody envelope[struct {
		ID         string `json:"id"`
		AccessRole string `json:"access_role"`
	}]
	decode(t, w, &albumBody)
	albumID := albumBody.Data.ID
	if albumID == "" || albumBody.Data.AccessRole != "owner" {
		t.Fatalf("album = %+v, want owner", albumBody.Data)
	}

	// An outsider does not even know it exists.
	expectErrorCode(t, h.do(http.MethodGet, "/api/v1/albums/"+albumID, marta, ""), http.StatusNotFound, "not_found")

	// Invite as viewer and accept.
	expectStatus(t, h.do(http.MethodPost, "/api/v1/albums/"+albumID+"/members", ana, `{"email":"luis@example.com","role":"viewer"}`), http.StatusCreated)
	expectErrorCode(t, h.do(http.MethodGet, "/api/v1/albums/"+albumID, luis, ""), http.StatusNotFound, "not_found")

	w = h.do(http.MethodGet, "/api/v1/invitations", luis, "")
	expectStatus(t, w, http.StatusOK)
	var invites envelope[[]struct {
		AlbumID string `json:"album_id"`
	}]
	decode(t, w, &invites)
	if len(invites.Data) != 1 || invites.Data[0].AlbumID != albumID {
		t.Fatalf("invitations = %+v, want the album", invites.Data)
	}

	expectStatus(t, h.do(http.MethodPost, "/api/v1/invitations/"+albumID+"/accept", luis, ""), http.StatusOK)
	expectStatus(t, h.do(http.MethodGet, "/api/v1/albums/"+albumID, luis, ""), http.StatusOK)

	// Viewer cannot upload or edit.
	w = h.do(http.MethodPost, "/api/v1/albums/"+albumID+"/photos/uploads", luis, `{"filename":"a.png","content_type":"image/png","size_bytes":4}`)
	expectErrorCode(t, w, http.StatusForbidden, "forbidden")
	expectErrorCode(t, h.do(http.MethodPatch, "/api/v1/albums/"+albumID, luis, `{"title":"Hack"}`), http.StatusForbidden, "forbidden")

	// Promote to editor.
	expectStatus(t, h.do(http.MethodPatch, "/api/v1/albums/"+albumID+"/members/"+luisID, ana, `{"role":"editor"}`), http.StatusNoContent)

	// Upload flow: ticket, client PUT (simulated), complete, download.
	w = h.do(http.MethodPost, "/api/v1/albums/"+albumID+"/photos/uploads", luis, `{"filename":"foto.png","content_type":"image/png","size_bytes":4}`)
	expectStatus(t, w, http.StatusCreated)
	var ticket envelope[struct {
		PhotoID   string `json:"photo_id"`
		UploadURL string `json:"upload_url"`
	}]
	decode(t, w, &ticket)
	if ticket.Data.PhotoID == "" || ticket.Data.UploadURL == "" {
		t.Fatalf("ticket = %+v, want photo id and url", ticket.Data)
	}

	key, ok := h.photos.ObjectKey(ticket.Data.PhotoID)
	if !ok {
		t.Fatal("photo was not persisted")
	}
	h.blobs.Put(key, domain.BlobInfo{Size: 4, ContentType: "image/png"})

	w = h.do(http.MethodPost, "/api/v1/albums/"+albumID+"/photos/"+ticket.Data.PhotoID+"/complete", luis, `{"width":1,"height":1}`)
	expectStatus(t, w, http.StatusOK)
	var photoBody envelope[struct {
		Status string `json:"status"`
	}]
	decode(t, w, &photoBody)
	if photoBody.Data.Status != "ready" {
		t.Fatalf("photo status = %q, want ready", photoBody.Data.Status)
	}

	w = h.do(http.MethodGet, "/api/v1/photos/"+ticket.Data.PhotoID+"/download-url", luis, "")
	expectStatus(t, w, http.StatusOK)
	var download envelope[struct {
		URL string `json:"url"`
	}]
	decode(t, w, &download)
	if download.Data.URL == "" {
		t.Fatal("download url is empty")
	}

	// Outsiders cannot list photos or download them.
	expectErrorCode(t, h.do(http.MethodGet, "/api/v1/albums/"+albumID+"/photos", marta, ""), http.StatusNotFound, "not_found")
	expectErrorCode(t, h.do(http.MethodGet, "/api/v1/photos/"+ticket.Data.PhotoID+"/download-url", marta, ""), http.StatusNotFound, "not_found")

	// Owner sets the cover.
	w = h.do(http.MethodPut, "/api/v1/albums/"+albumID+"/cover", ana, `{"photo_id":"`+ticket.Data.PhotoID+`"}`)
	expectStatus(t, w, http.StatusOK)
	var cover envelope[struct {
		CoverPhotoID *string `json:"cover_photo_id"`
	}]
	decode(t, w, &cover)
	if cover.Data.CoverPhotoID == nil || *cover.Data.CoverPhotoID != ticket.Data.PhotoID {
		t.Fatalf("cover = %v, want the uploaded photo", cover.Data.CoverPhotoID)
	}

	// Delete the photo; the download link disappears.
	expectStatus(t, h.do(http.MethodDelete, "/api/v1/photos/"+ticket.Data.PhotoID, ana, ""), http.StatusNoContent)
	expectErrorCode(t, h.do(http.MethodGet, "/api/v1/photos/"+ticket.Data.PhotoID+"/download-url", luis, ""), http.StatusNotFound, "not_found")

	// The member leaves and loses access; the owner deletes the album.
	expectStatus(t, h.do(http.MethodDelete, "/api/v1/albums/"+albumID+"/members/"+luisID, luis, ""), http.StatusNoContent)
	expectErrorCode(t, h.do(http.MethodGet, "/api/v1/albums/"+albumID, luis, ""), http.StatusNotFound, "not_found")
	expectStatus(t, h.do(http.MethodDelete, "/api/v1/albums/"+albumID, ana, ""), http.StatusNoContent)
	expectErrorCode(t, h.do(http.MethodGet, "/api/v1/albums/"+albumID, ana, ""), http.StatusNotFound, "not_found")
}

func TestInviteRequiresAnExistingAccount(t *testing.T) {
	h := newHarness(t)
	ana, tokenAna := h.register(t, "Ana", "ana@example.com", "supersecret")

	w := h.do(http.MethodPost, "/api/v1/albums", tokenAna, `{"title":"Viaje"}`)
	expectStatus(t, w, http.StatusCreated)
	var body envelope[struct {
		ID string `json:"id"`
	}]
	decode(t, w, &body)

	w = h.do(http.MethodPost, "/api/v1/albums/"+body.Data.ID+"/members", tokenAna, `{"email":"unknown@example.com"}`)
	expectStatus(t, w, http.StatusUnprocessableEntity)
	var errBody errorEnvelope
	decode(t, w, &errBody)
	if !strings.Contains(errBody.Error.Message, "must have an account") {
		t.Fatalf("message = %q, want account hint", errBody.Error.Message)
	}
	_ = ana
}

func TestAdminEndpoints(t *testing.T) {
	h := newHarness(t)

	adminID, adminToken := h.register(t, "Admin", "admin@example.com", "supersecret")
	h.promote(t, adminID)
	regularID, regularToken := h.register(t, "Ana", "ana@example.com", "supersecret")

	// Regular users cannot reach the admin module.
	expectErrorCode(t, h.do(http.MethodGet, "/api/v1/admin/users", regularToken, ""), http.StatusForbidden, "forbidden")
	expectErrorCode(t, h.do(http.MethodGet, "/api/v1/admin/users", "", ""), http.StatusUnauthorized, "unauthorized")

	// Admins can list and search users.
	w := h.do(http.MethodGet, "/api/v1/admin/users?page=1&per_page=1&search=ana", adminToken, "")
	expectStatus(t, w, http.StatusOK)
	var list envelope[struct {
		Items []domain.User `json:"items"`
		Meta  struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
			Total   int `json:"total"`
		} `json:"meta"`
	}]
	decode(t, w, &list)
	if list.Data.Meta.Total != 1 || len(list.Data.Items) != 1 || list.Data.Items[0].ID != regularID {
		t.Fatalf("admin list = %+v, want only Ana", list.Data)
	}

	// Admin can promote a user. The existing token picks up the new flag because
	// the user is reloaded on every request.
	w = h.do(http.MethodPatch, "/api/v1/admin/users/"+regularID, adminToken, `{"is_admin":true}`)
	expectStatus(t, w, http.StatusOK)
	var promoted envelope[domain.User]
	decode(t, w, &promoted)
	if !promoted.Data.IsAdmin {
		t.Fatal("admin update did not set is_admin")
	}
	expectStatus(t, h.do(http.MethodGet, "/api/v1/admin/users", regularToken, ""), http.StatusOK)

	// Admins cannot revoke or delete themselves.
	expectErrorCode(t, h.do(http.MethodPatch, "/api/v1/admin/users/"+adminID, adminToken, `{"is_admin":false}`), http.StatusUnprocessableEntity, "invalid")
	expectErrorCode(t, h.do(http.MethodDelete, "/api/v1/admin/users/"+adminID, adminToken, ""), http.StatusUnprocessableEntity, "invalid")

	// Deleting a user invalidates their token immediately.
	expectStatus(t, h.do(http.MethodDelete, "/api/v1/admin/users/"+regularID, adminToken, ""), http.StatusNoContent)
	expectErrorCode(t, h.do(http.MethodGet, "/api/v1/auth/me", regularToken, ""), http.StatusUnauthorized, "unauthorized")
}

func TestCORS(t *testing.T) {
	h := newHarness(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/albums", nil)
	req.Header.Set("Origin", "http://app.test")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://app.test" {
		t.Fatalf("Allow-Origin = %q, want the request origin", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Authorization") {
		t.Fatalf("Allow-Headers = %q, want Authorization", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://evil.test")
	w = httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Allow-Origin for a foreign origin = %q, want empty", got)
	}
}
