package auth

import (
    "database/sql"
    "time"
    "strings"
    "net/http"

    "emby-analytics/internal/config"
    "emby-analytics/internal/logging"

    "github.com/gofiber/fiber/v3"
    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
)

type userRow struct {
    ID       int64
    Username string
    Role     string
}

func getUserByUsername(db *sql.DB, username string) (*userRow, string, error) {
    var u userRow
    var hash string
    err := db.QueryRow(`SELECT id, username, role, password_hash FROM app_user WHERE lower(username)=lower(?)`, username).Scan(&u.ID, &u.Username, &u.Role, &hash)
    if err != nil {
        return nil, "", err
    }
    return &u, hash, nil
}

func insertUser(db *sql.DB, username, passwordHash, role string) (int64, error) {
    res, err := db.Exec(`INSERT INTO app_user (username, password_hash, role) VALUES (?, ?, ?)`, username, passwordHash, role)
    if err != nil {
        return 0, err
    }
    return res.LastInsertId()
}

func countUsers(db *sql.DB) (int64, error) {
    var n int64
    err := db.QueryRow(`SELECT COUNT(*) FROM app_user`).Scan(&n)
    return n, err
}

func upsertSession(db *sql.DB, userID int64, ttl time.Duration) (string, time.Time, error) {
    token := uuid.NewString()
    expires := time.Now().Add(ttl)
    _, err := db.Exec(`INSERT INTO app_session (token, user_id, expires_at) VALUES (?, ?, ?)`, token, userID, expires.UTC())
    if err != nil {
        return "", time.Time{}, err
    }
    return token, expires, nil
}

func deleteSession(db *sql.DB, token string) {
    _, _ = db.Exec(`DELETE FROM app_session WHERE token=?`, token)
}

func findSessionUser(db *sql.DB, token string) (*userRow, error) {
    var u userRow
    var expires time.Time
    err := db.QueryRow(`
        SELECT u.id, u.username, u.role, s.expires_at
        FROM app_session s
        JOIN app_user u ON u.id = s.user_id
        WHERE s.token = ?
    `, token).Scan(&u.ID, &u.Username, &u.Role, &expires)
    if err != nil {
        return nil, err
    }
    if time.Now().After(expires) {
        // session expired; cleanup
        go deleteSession(db, token)
        return nil, sql.ErrNoRows
    }
    return &u, nil
}

type loginReq struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

func LoginHandler(db *sql.DB, cfg config.Config) fiber.Handler {
    return func(c fiber.Ctx) error {
        var req loginReq
        if err := c.Bind().Body(&req); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
        }
        req.Username = strings.TrimSpace(req.Username)
        if req.Username == "" || req.Password == "" {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username and password required"})
        }
        u, hash, err := getUserByUsername(db, req.Username)
        if err != nil {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
        }
        if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
        }
        token, exp, err := upsertSession(db, u.ID, time.Duration(cfg.AuthSessionTTLMinutes)*time.Minute)
        if err != nil {
            logging.Error("failed to create session", "error", err)
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "session error"})
        }
        setAuthCookie(c, cfg, token, exp)
        return c.JSON(fiber.Map{"user": fiber.Map{"id": u.ID, "username": u.Username, "role": u.Role}})
    }
}

func LogoutHandler(db *sql.DB, cfg config.Config) fiber.Handler {
    return func(c fiber.Ctx) error {
        if token := readAuthCookie(c, cfg); token != "" {
            deleteSession(db, token)
        }
        expireAuthCookie(c, cfg)
        return c.SendStatus(http.StatusNoContent)
    }
}

type registerReq struct {
    Username string `json:"username"`
    Password string `json:"password"`
    Secret   string `json:"secret"`
}

func RegisterHandler(db *sql.DB, cfg config.Config) fiber.Handler {
    return func(c fiber.Ctx) error {
        var req registerReq
        if err := c.Bind().Body(&req); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
        }
        req.Username = strings.TrimSpace(req.Username)
        if req.Username == "" || req.Password == "" {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username and password required"})
        }

        // Registration policy
        allowed, role := isRegistrationAllowed(db, cfg, c, req.Secret)
        if !allowed {
            return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "registration disabled"})
        }

        hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
        if err != nil {
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "hash error"})
        }
        uid, err := insertUser(db, req.Username, string(hash), role)
        if err != nil {
            return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "username taken"})
        }

        token, exp, err := upsertSession(db, uid, time.Duration(cfg.AuthSessionTTLMinutes)*time.Minute)
        if err != nil {
            logging.Error("failed to create session", "error", err)
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "session error"})
        }
        setAuthCookie(c, cfg, token, exp)
        return c.Status(http.StatusCreated).JSON(fiber.Map{"user": fiber.Map{"id": uid, "username": req.Username, "role": role}})
    }
}

func MeHandler(db *sql.DB, cfg config.Config) fiber.Handler {
    return func(c fiber.Ctx) error {
        token := readAuthCookie(c, cfg)
        if token == "" {
            return c.SendStatus(http.StatusUnauthorized)
        }
        u, err := findSessionUser(db, token)
        if err != nil {
            return c.SendStatus(http.StatusUnauthorized)
        }
        return c.JSON(fiber.Map{"id": u.ID, "username": u.Username, "role": u.Role})
    }
}

type AuthConfig struct {
    Enabled            bool   `json:"enabled"`
    RegistrationMode   string `json:"registration_mode"`
    RegistrationOpen   bool   `json:"registration_open"`
    RequiresSecret     bool   `json:"requires_secret"`
}

func ConfigHandler(db *sql.DB, cfg config.Config) fiber.Handler {
    return func(c fiber.Ctx) error {
        n, _ := countUsers(db)
        mode := strings.ToLower(cfg.AuthRegistrationMode)
        open := false
        requiresSecret := false
        switch mode {
        case "open":
            open = true
        case "secret":
            if n == 0 {
                open = true // bootstrap
            } else {
                requiresSecret = true
                open = true // allowed with secret
            }
        default:
            if n == 0 {
                open = true // bootstrap
            }
        }
        return c.JSON(AuthConfig{
            Enabled:          cfg.AuthEnabled,
            RegistrationMode: mode,
            RegistrationOpen: open,
            RequiresSecret:   requiresSecret,
        })
    }
}

func isRegistrationAllowed(db *sql.DB, cfg config.Config, c fiber.Ctx, providedSecret string) (bool, string) {
    // First user bootstrap: if no users exist, allow and grant admin role
    if n, err := countUsers(db); err == nil && n == 0 {
        return true, "admin"
    }
    mode := strings.ToLower(cfg.AuthRegistrationMode)
    switch mode {
    case "open":
        return true, "user"
    case "secret":
        // Accept secret via header or body field
        if providedSecret == "" {
            providedSecret = c.Get("X-Registration-Secret")
        }
        if providedSecret != "" && providedSecret == cfg.AuthRegistrationSecret {
            return true, "user"
        }
        return false, ""
    default: // "closed"
        return false, ""
    }
}

func readAuthCookie(c fiber.Ctx, cfg config.Config) string {
    return c.Cookies(cfg.AuthCookieName)
}

func setAuthCookie(c fiber.Ctx, cfg config.Config, token string, exp time.Time) {
    c.Cookie(&fiber.Cookie{
        Name:     cfg.AuthCookieName,
        Value:    token,
        Expires:  exp,
        HTTPOnly: true,
        Secure:   false,
        SameSite: fiber.CookieSameSiteLaxMode,
        Path:     "/",
    })
}

func expireAuthCookie(c fiber.Ctx, cfg config.Config) {
    c.Cookie(&fiber.Cookie{
        Name:     cfg.AuthCookieName,
        Value:    "",
        Expires:  time.Unix(0, 0),
        HTTPOnly: true,
        Secure:   false,
        SameSite: fiber.CookieSameSiteLaxMode,
        Path:     "/",
    })
}
