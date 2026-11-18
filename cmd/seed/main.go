// Command seed creates idempotent demo data for local development.
//
// It talks directly to Postgres (no MinIO required), resets the demo
// credentials and guarantees an administrator account. It refuses to run
// against production unless -force is passed.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"
	"time"

	"api-hamy/internal/domain"
	"api-hamy/internal/repository/postgres"
	authservice "api-hamy/internal/service/auth"
)

const (
	demoAlbumTitle    = "Escapada a la playa"
	demoAlbumDesc     = "Álbum de ejemplo creado por make seed"
	defaultUserPass   = "demo-password"
	defaultAdminPass  = "admin-password"
	defaultAdminEmail = "admin@example.com"
	defaultAdminName  = "Admin"
)

type demoUser struct {
	name     string
	email    string
	password string
	isAdmin  bool
}

func main() {
	force := flag.Bool("force", false, "run even when APP_ENV=production")
	flag.Parse()

	appEnv := getenv("APP_ENV", "development")
	if appEnv == "production" && !*force {
		fmt.Fprintln(os.Stderr, "refusing to seed demo data with APP_ENV=production (use -force to override)")
		os.Exit(1)
	}

	userPass := getenv("SEED_USER_PASSWORD", defaultUserPass)
	adminPass := getenv("SEED_ADMIN_PASSWORD", defaultAdminPass)
	if len(userPass) < 8 || len(adminPass) < 8 {
		fatal("invalid seed configuration", errors.New("SEED_USER_PASSWORD and SEED_ADMIN_PASSWORD must be at least 8 characters"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dsn := getenv("DB_DSN", "postgres://hamy:hamy@localhost:5432/hamy?sslmode=disable")
	pool, err := postgres.NewPool(ctx, dsn, 4)
	if err != nil {
		fatal("could not connect to postgres", err)
	}
	defer pool.Close()

	users := postgres.NewUserRepository(pool)
	albums := postgres.NewAlbumRepository(pool)

	seedUsers := []demoUser{
		{name: defaultAdminName, email: defaultAdminEmail, password: adminPass, isAdmin: true},
		{name: "Ana", email: "ana@example.com", password: userPass},
		{name: "Luis", email: "luis@example.com", password: userPass},
		{name: "Marta", email: "marta@example.com", password: userPass},
	}

	created := make(map[string]*domain.User, len(seedUsers))
	for _, demo := range seedUsers {
		u, err := ensureUser(ctx, users, demo)
		if err != nil {
			fatal("could not seed user "+demo.email, err)
		}
		created[demo.email] = u
	}

	ana := created["ana@example.com"]
	luis := created["luis@example.com"]
	marta := created["marta@example.com"]
	admin := created[defaultAdminEmail]

	album, err := ensureAlbum(ctx, albums, ana, demoAlbumTitle)
	if err != nil {
		fatal("could not seed album", err)
	}

	if err := ensureMember(ctx, albums, album.ID, ana.ID, luis.ID, domain.RoleEditor, domain.StatusActive); err != nil {
		fatal("could not add Luis to the album", err)
	}
	if err := ensureMember(ctx, albums, album.ID, ana.ID, marta.ID, domain.RoleViewer, domain.StatusInvited); err != nil {
		fatal("could not invite Marta to the album", err)
	}

	printSummary(created, album, admin, userPass, adminPass)
}

// ensureUser creates the demo user or resets it to a known state. Resetting the
// password on every run keeps the demo credentials predictable and also fixes
// phase-0 users, which were created without a password_hash.
func ensureUser(ctx context.Context, repo *postgres.UserRepository, demo demoUser) (*domain.User, error) {
	hash, err := authservice.HashPassword(demo.password)
	if err != nil {
		return nil, err
	}

	u, err := repo.GetByEmail(ctx, demo.email)
	if errors.Is(err, domain.ErrNotFound) {
		u = &domain.User{Name: demo.name, Email: demo.email}
		u.PasswordHash = hash
		u.IsAdmin = demo.isAdmin
		if err := repo.Create(ctx, u); err != nil {
			return nil, err
		}
		return u, nil
	}
	if err != nil {
		return nil, err
	}

	u.Name = demo.name
	u.PasswordHash = hash
	u.IsAdmin = demo.isAdmin
	if err := repo.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func ensureAlbum(ctx context.Context, repo *postgres.AlbumRepository, owner *domain.User, title string) (*domain.Album, error) {
	existing, _, err := repo.ListForActor(ctx, owner.ID, domain.ListFilter{Page: 1, PerPage: 100})
	if err != nil {
		return nil, err
	}
	for i := range existing {
		if existing[i].Title == title {
			a := existing[i]
			return &a, nil
		}
	}

	description := demoAlbumDesc
	a := &domain.Album{OwnerID: owner.ID, Title: title, Description: &description}
	if err := repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func ensureMember(ctx context.Context, repo *postgres.AlbumRepository, albumID, invitedBy, userID string, role domain.MemberRole, status domain.MemberStatus) error {
	_, err := repo.GetMember(ctx, albumID, userID)
	switch {
	case err == nil:
		// Upsert keeps the seed idempotent and restores the demo state.
	case errors.Is(err, domain.ErrNotFound):
	default:
		return err
	}

	_, err = repo.UpsertMember(ctx, &domain.AlbumMember{
		AlbumID:   albumID,
		UserID:    userID,
		Role:      role,
		Status:    status,
		InvitedBy: &invitedBy,
	})
	return err
}

func printSummary(users map[string]*domain.User, album *domain.Album, admin *domain.User, userPass, adminPass string) {
	fmt.Println()
	fmt.Println("Demo accounts (credentials are reset on every seed):")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "EMAIL\tPASSWORD\tROLE")
	fmt.Fprintf(w, "%s\t%s\tadmin\n", admin.Email, adminPass)
	fmt.Fprintf(w, "%s\t%s\towner of the demo album\n", users["ana@example.com"].Email, userPass)
	fmt.Fprintf(w, "%s\t%s\teditor (active)\n", users["luis@example.com"].Email, userPass)
	fmt.Fprintf(w, "%s\t%s\tviewer (invited)\n", users["marta@example.com"].Email, userPass)
	w.Flush()

	fmt.Printf("\nAlbum: %s (%s)\n", album.Title, album.ID)
	fmt.Printf("\nTry:\n")
	fmt.Printf("  TOKEN=$(curl -s localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' \\\n")
	fmt.Printf("    -d '{\"email\":\"ana@example.com\",\"password\":\"%s\"}' | jq -r .data.access_token)\n", userPass)
	fmt.Printf("  curl -H \"Authorization: Bearer $TOKEN\" localhost:8080/api/v1/albums\n")
	fmt.Printf("\nAdmin:\n  curl -s localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' \\\n")
	fmt.Printf("    -d '{\"email\":\"%s\",\"password\":\"%s\"}'\n", admin.Email, adminPass)
	fmt.Printf("\nInvitations are pending until the invitee calls POST /api/v1/invitations/%s/accept.\n", album.ID)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fatal(message string, err error) {
	slog.Error(message, "err", err)
	os.Exit(1)
}
