// Package testutil provides in-memory implementations of the domain ports.
// They mirror the SQL semantics (album_access, soft deletes, state machines)
// so service and transport tests can run without Postgres or MinIO.
package testutil

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"api-hamy/internal/domain"
)

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

// FakeUserRepo is an in-memory domain.UserRepository.
type FakeUserRepo struct {
	mu    sync.Mutex
	users map[string]*domain.User
}

var _ domain.UserRepository = (*FakeUserRepo)(nil)

func NewFakeUserRepo() *FakeUserRepo {
	return &FakeUserRepo{users: make(map[string]*domain.User)}
}

// Add creates an active user bypassing validation and returns a copy.
func (r *FakeUserRepo) Add(name, email string) *domain.User {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	u := &domain.User{
		ID:        uuid.NewString(),
		Name:      name,
		Email:     strings.ToLower(email),
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.users[u.ID] = u
	cp := *u
	return &cp
}

// AddAdmin creates an active admin user.
func (r *FakeUserRepo) AddAdmin(name, email string) *domain.User {
	u := r.Add(name, email)

	r.mu.Lock()
	defer r.mu.Unlock()

	if stored, ok := r.users[u.ID]; ok {
		stored.IsAdmin = true
		cp := *stored
		return &cp
	}
	return u
}

func (r *FakeUserRepo) Create(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.users {
		if existing.DeletedAt == nil && strings.EqualFold(existing.Email, u.Email) {
			return domain.ErrConflict
		}
	}

	now := time.Now().UTC()
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	u.CreatedAt = now
	u.UpdatedAt = now

	cp := *u
	r.users[u.ID] = &cp
	return nil
}

func (r *FakeUserRepo) GetByID(_ context.Context, id string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.users[id]
	if !ok || u.DeletedAt != nil {
		return nil, domain.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (r *FakeUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, u := range r.users {
		if u.DeletedAt == nil && strings.EqualFold(u.Email, email) {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *FakeUserRepo) List(_ context.Context, f domain.ListFilter) ([]domain.User, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	f = f.Normalize()
	search := strings.ToLower(f.Search)

	matches := make([]domain.User, 0, len(r.users))
	for _, u := range r.users {
		if u.DeletedAt != nil {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(u.Name), search) && !strings.Contains(strings.ToLower(u.Email), search) {
			continue
		}
		matches = append(matches, *u)
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i].CreatedAt.After(matches[j].CreatedAt) })

	total := int64(len(matches))
	start := f.Offset()
	if start > len(matches) {
		start = len(matches)
	}
	end := start + f.PerPage
	if end > len(matches) {
		end = len(matches)
	}

	return matches[start:end], total, nil
}

func (r *FakeUserRepo) Update(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.users[u.ID]
	if !ok || current.DeletedAt != nil {
		return domain.ErrNotFound
	}
	for _, other := range r.users {
		if other.ID != u.ID && other.DeletedAt == nil && strings.EqualFold(other.Email, u.Email) {
			return domain.ErrConflict
		}
	}

	current.Name = u.Name
	current.Email = u.Email
	current.PasswordHash = u.PasswordHash
	current.IsAdmin = u.IsAdmin
	current.UpdatedAt = time.Now().UTC()
	u.UpdatedAt = current.UpdatedAt
	return nil
}

func (r *FakeUserRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.users[id]
	if !ok || u.DeletedAt != nil {
		return domain.ErrNotFound
	}
	now := time.Now().UTC()
	u.DeletedAt = &now
	return nil
}

// ---------------------------------------------------------------------------
// Albums and memberships
// ---------------------------------------------------------------------------

// FakeAlbumRepo is an in-memory domain.AlbumRepository.
//
// PhotoLookup is optional: when set, SetCover uses it to enforce the same rule
// as the SQL query (the photo must belong to the album and be ready).
type FakeAlbumRepo struct {
	mu          sync.Mutex
	albums      map[string]*domain.Album
	members     map[string]*domain.AlbumMember
	users       *FakeUserRepo
	PhotoLookup func(photoID string) (albumID string, ready bool, exists bool)
}

var _ domain.AlbumRepository = (*FakeAlbumRepo)(nil)

func NewFakeAlbumRepo(users *FakeUserRepo) *FakeAlbumRepo {
	return &FakeAlbumRepo{
		albums:  make(map[string]*domain.Album),
		members: make(map[string]*domain.AlbumMember),
		users:   users,
	}
}

func memberKey(albumID, userID string) string { return albumID + "|" + userID }

func (r *FakeAlbumRepo) access(albumID, userID string) (domain.MemberRole, bool) {
	a, ok := r.albums[albumID]
	if !ok || a.DeletedAt != nil {
		return "", false
	}
	if a.OwnerID == userID {
		return domain.RoleOwner, true
	}
	m, ok := r.members[memberKey(albumID, userID)]
	if !ok || m.Status != domain.StatusActive {
		return "", false
	}
	return m.Role, true
}

// AccessRole exposes the effective role for callers such as FakePhotoRepo.
func (r *FakeAlbumRepo) AccessRole(albumID, userID string) (domain.MemberRole, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.access(albumID, userID)
}

func (r *FakeAlbumRepo) Create(_ context.Context, a *domain.Album) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	a.CreatedAt = now
	a.UpdatedAt = now
	a.AccessRole = domain.RoleOwner

	cp := *a
	r.albums[a.ID] = &cp
	return nil
}

func (r *FakeAlbumRepo) GetForActor(_ context.Context, albumID, actorID string) (*domain.Album, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	role, ok := r.access(albumID, actorID)
	if !ok {
		return nil, domain.ErrNotFound
	}

	cp := *r.albums[albumID]
	cp.AccessRole = role
	return &cp, nil
}

func (r *FakeAlbumRepo) ListForActor(_ context.Context, actorID string, f domain.ListFilter) ([]domain.Album, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	f = f.Normalize()
	search := strings.ToLower(f.Search)

	matches := make([]domain.Album, 0, len(r.albums))
	for id, a := range r.albums {
		role, ok := r.access(id, actorID)
		if !ok {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(a.Title), search) {
			continue
		}
		cp := *a
		cp.AccessRole = role
		matches = append(matches, cp)
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i].UpdatedAt.After(matches[j].UpdatedAt) })

	total := int64(len(matches))
	start := f.Offset()
	if start > len(matches) {
		start = len(matches)
	}
	end := start + f.PerPage
	if end > len(matches) {
		end = len(matches)
	}

	return matches[start:end], total, nil
}

func (r *FakeAlbumRepo) Update(_ context.Context, albumID, actorID string, title, description *string, clearDescription bool) (*domain.Album, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.albums[albumID]
	if !ok || a.DeletedAt != nil || a.OwnerID != actorID {
		return nil, domain.ErrNotFound
	}

	if title != nil {
		a.Title = *title
	}
	if clearDescription {
		a.Description = nil
	} else if description != nil {
		a.Description = description
	}
	a.UpdatedAt = time.Now().UTC()

	cp := *a
	cp.AccessRole = domain.RoleOwner
	return &cp, nil
}

func (r *FakeAlbumRepo) SetCover(_ context.Context, albumID, actorID string, photoID *string) (*domain.Album, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.albums[albumID]
	if !ok || a.DeletedAt != nil || a.OwnerID != actorID {
		return nil, domain.ErrNotFound
	}

	if photoID != nil && r.PhotoLookup != nil {
		photoAlbum, ready, exists := r.PhotoLookup(*photoID)
		if !exists || !ready || photoAlbum != albumID {
			return nil, domain.ErrNotFound
		}
	}

	a.CoverPhotoID = photoID
	a.UpdatedAt = time.Now().UTC()

	cp := *a
	cp.AccessRole = domain.RoleOwner
	return &cp, nil
}

func (r *FakeAlbumRepo) Delete(_ context.Context, albumID, actorID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.albums[albumID]
	if !ok || a.DeletedAt != nil || a.OwnerID != actorID {
		return domain.ErrNotFound
	}
	now := time.Now().UTC()
	a.DeletedAt = &now
	return nil
}

func (r *FakeAlbumRepo) ClearCoverForPhoto(_ context.Context, photoID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, a := range r.albums {
		if a.CoverPhotoID != nil && *a.CoverPhotoID == photoID {
			a.CoverPhotoID = nil
		}
	}
	return nil
}

func (r *FakeAlbumRepo) UpsertMember(_ context.Context, m *domain.AlbumMember) (*domain.AlbumMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := memberKey(m.AlbumID, m.UserID)
	now := time.Now().UTC()

	current, ok := r.members[key]
	if !ok {
		current = &domain.AlbumMember{AlbumID: m.AlbumID, UserID: m.UserID, CreatedAt: now}
		r.members[key] = current
	}
	current.Role = m.Role
	current.Status = m.Status
	current.InvitedBy = m.InvitedBy
	current.UpdatedAt = now

	cp := *current
	r.fillMemberUser(&cp)
	return &cp, nil
}

func (r *FakeAlbumRepo) GetMember(_ context.Context, albumID, userID string) (*domain.AlbumMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.members[memberKey(albumID, userID)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *m
	r.fillMemberUser(&cp)
	return &cp, nil
}

func (r *FakeAlbumRepo) ListMembers(_ context.Context, albumID, actorID string) ([]domain.AlbumMember, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.access(albumID, actorID); !ok {
		return nil, domain.ErrNotFound
	}

	out := make([]domain.AlbumMember, 0)
	for _, m := range r.members {
		if m.AlbumID != albumID || m.Status == domain.StatusRevoked {
			continue
		}
		cp := *m
		r.fillMemberUser(&cp)
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (r *FakeAlbumRepo) ListInvitations(_ context.Context, userID string) ([]domain.AlbumInvitation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]domain.AlbumInvitation, 0)
	for _, m := range r.members {
		if m.UserID != userID || m.Status != domain.StatusInvited {
			continue
		}
		a, ok := r.albums[m.AlbumID]
		if !ok || a.DeletedAt != nil {
			continue
		}

		inv := domain.AlbumInvitation{
			AlbumID:    a.ID,
			AlbumTitle: a.Title,
			UserID:     m.UserID,
			Role:       m.Role,
			Status:     m.Status,
			CreatedAt:  m.CreatedAt,
		}
		if m.InvitedBy != nil && r.users != nil {
			if u := r.users.mustGet(*m.InvitedBy); u != nil {
				name, email := u.Name, u.Email
				inv.InvitedByName = &name
				inv.InvitedByEmail = &email
			}
		}
		out = append(out, inv)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *FakeAlbumRepo) ActivateMember(_ context.Context, albumID, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.members[memberKey(albumID, userID)]
	if !ok || m.Status != domain.StatusInvited {
		return domain.ErrNotFound
	}
	m.Status = domain.StatusActive
	m.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *FakeAlbumRepo) UpdateMemberRole(_ context.Context, albumID, userID string, role domain.MemberRole) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.members[memberKey(albumID, userID)]
	if !ok || m.Status == domain.StatusRevoked {
		return domain.ErrNotFound
	}
	m.Role = role
	m.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *FakeAlbumRepo) RevokeMember(_ context.Context, albumID, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.members[memberKey(albumID, userID)]
	if !ok || m.Status == domain.StatusRevoked {
		return domain.ErrNotFound
	}
	m.Status = domain.StatusRevoked
	m.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *FakeAlbumRepo) fillMemberUser(m *domain.AlbumMember) {
	if r.users == nil {
		return
	}
	if u := r.users.mustGet(m.UserID); u != nil {
		m.UserName = u.Name
		m.UserEmail = u.Email
	}
}

// mustGet returns a copy of the user or nil. It is a test helper.
func (r *FakeUserRepo) mustGet(id string) *domain.User {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.users[id]
	if !ok {
		return nil
	}
	cp := *u
	return &cp
}

// FindByEmail is a test helper that returns nil when the email is unknown.
func (r *FakeUserRepo) FindByEmail(email string) *domain.User {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, u := range r.users {
		if u.DeletedAt == nil && strings.EqualFold(u.Email, email) {
			cp := *u
			return &cp
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Photos
// ---------------------------------------------------------------------------

// FakePhotoRepo is an in-memory domain.PhotoRepository.
type FakePhotoRepo struct {
	mu     sync.Mutex
	photos map[string]*domain.Photo
	albums *FakeAlbumRepo
}

var _ domain.PhotoRepository = (*FakePhotoRepo)(nil)

func NewFakePhotoRepo(albums *FakeAlbumRepo) *FakePhotoRepo {
	return &FakePhotoRepo{photos: make(map[string]*domain.Photo), albums: albums}
}

func (r *FakePhotoRepo) CreatePending(_ context.Context, albumID, uploadedBy, bucket, filename, contentType string, size int64) (*domain.Photo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	role, ok := r.albums.access(albumID, uploadedBy)
	if !ok || !role.CanEdit() {
		return nil, domain.ErrNotFound
	}

	now := time.Now().UTC()
	id := uuid.NewString()
	p := &domain.Photo{
		ID:               id,
		AlbumID:          albumID,
		UploadedBy:       &uploadedBy,
		Bucket:           bucket,
		ObjectKey:        "albums/" + albumID + "/" + id,
		OriginalFilename: filename,
		ContentType:      contentType,
		SizeBytes:        size,
		Status:           domain.PhotoPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	r.photos[id] = p

	cp := *p
	return &cp, nil
}

func (r *FakePhotoRepo) GetForActor(_ context.Context, photoID, actorID string) (*domain.Photo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.photos[photoID]
	if !ok || p.DeletedAt != nil {
		return nil, domain.ErrNotFound
	}
	if _, ok := r.albums.access(p.AlbumID, actorID); !ok {
		return nil, domain.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (r *FakePhotoRepo) GetForEditor(_ context.Context, photoID, actorID string) (*domain.Photo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.photos[photoID]
	if !ok || p.DeletedAt != nil {
		return nil, domain.ErrNotFound
	}
	role, ok := r.albums.access(p.AlbumID, actorID)
	if !ok {
		return nil, domain.ErrNotFound
	}
	if !role.CanEdit() && (p.UploadedBy == nil || *p.UploadedBy != actorID) {
		return nil, domain.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (r *FakePhotoRepo) ListForActor(_ context.Context, albumID, actorID string, f domain.ListFilter) ([]domain.Photo, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.albums.access(albumID, actorID); !ok {
		return nil, 0, domain.ErrNotFound
	}

	f = f.Normalize()
	search := strings.ToLower(f.Search)

	matches := make([]domain.Photo, 0)
	for _, p := range r.photos {
		if p.AlbumID != albumID || p.DeletedAt != nil || p.Status != domain.PhotoReady {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(p.OriginalFilename), search) {
			continue
		}
		matches = append(matches, *p)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].CreatedAt.After(matches[j].CreatedAt) })

	total := int64(len(matches))
	start := f.Offset()
	if start > len(matches) {
		start = len(matches)
	}
	end := start + f.PerPage
	if end > len(matches) {
		end = len(matches)
	}
	return matches[start:end], total, nil
}

func (r *FakePhotoRepo) MarkReady(_ context.Context, photoID string, width, height *int32, checksum *string) (*domain.Photo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.photos[photoID]
	if !ok || p.DeletedAt != nil || p.Status != domain.PhotoPending {
		return nil, domain.ErrNotFound
	}
	p.Status = domain.PhotoReady
	if width != nil {
		p.Width = width
	}
	if height != nil {
		p.Height = height
	}
	if checksum != nil {
		p.ChecksumSHA256 = checksum
	}
	p.UpdatedAt = time.Now().UTC()

	cp := *p
	return &cp, nil
}

func (r *FakePhotoRepo) SoftDelete(_ context.Context, photoID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.photos[photoID]
	if !ok || p.DeletedAt != nil {
		return domain.ErrNotFound
	}
	now := time.Now().UTC()
	p.DeletedAt = &now
	return nil
}

func (r *FakePhotoRepo) ListStalePending(_ context.Context, before time.Time, limit int32) ([]domain.Photo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]domain.Photo, 0)
	for _, p := range r.photos {
		if p.Status == domain.PhotoPending && p.DeletedAt == nil && p.CreatedAt.Before(before) {
			out = append(out, *p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	if int32(len(out)) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Get returns a copy of a photo regardless of access, for test fixtures.
func (r *FakePhotoRepo) Get(photoID string) (*domain.Photo, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.photos[photoID]
	if !ok {
		return nil, false
	}
	cp := *p
	return &cp, true
}

// ObjectKey returns the storage key of a photo, for tests that simulate the
// client PUT against the fake blob store.
func (r *FakePhotoRepo) ObjectKey(photoID string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.photos[photoID]
	if !ok {
		return "", false
	}
	return p.ObjectKey, true
}

// ---------------------------------------------------------------------------
// Blob store
// ---------------------------------------------------------------------------

// FakeBlobStore is an in-memory domain.BlobStore.
type FakeBlobStore struct {
	mu      sync.Mutex
	objects map[string]domain.BlobInfo

	// PresignErr, when set, makes presigning fail.
	PresignErr error
}

var _ domain.BlobStore = (*FakeBlobStore)(nil)

func NewFakeBlobStore() *FakeBlobStore {
	return &FakeBlobStore{objects: make(map[string]domain.BlobInfo)}
}

func (b *FakeBlobStore) PresignPut(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	if b.PresignErr != nil {
		return "", b.PresignErr
	}
	return "http://blob.test/" + url.PathEscape(key) + "?method=put", nil
}

func (b *FakeBlobStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	if b.PresignErr != nil {
		return "", b.PresignErr
	}
	return "http://blob.test/" + url.PathEscape(key) + "?method=get", nil
}

func (b *FakeBlobStore) Stat(_ context.Context, key string) (domain.BlobInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	info, ok := b.objects[key]
	if !ok {
		return domain.BlobInfo{}, domain.ErrBlobNotFound
	}
	return info, nil
}

func (b *FakeBlobStore) Remove(_ context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.objects, key)
	return nil
}

// Put simulates a completed client upload.
func (b *FakeBlobStore) Put(key string, info domain.BlobInfo) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.objects[key] = info
}

// Has reports whether the object exists.
func (b *FakeBlobStore) Has(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.objects[key]
	return ok
}

// ObjectCount returns the number of stored objects, for assertions.
func (b *FakeBlobStore) ObjectCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.objects)
}

// ErrNotFoundf is a small helper to build domain errors in tests.
func ErrNotFoundf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{domain.ErrNotFound}, args...)...)
}
