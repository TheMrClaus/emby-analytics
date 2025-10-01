package auth

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
)

type AppUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

func ListAppUsers(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		rows, err := db.Query(`SELECT id, username, role, COALESCE(strftime('%Y-%m-%dT%H:%M:%fZ', created_at), '') as created_at FROM app_user ORDER BY id ASC`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()
		out := make([]AppUser, 0, 8)
		for rows.Next() {
			var u AppUser
			if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err == nil {
				out = append(out, u)
			}
		}
		return c.JSON(out)
	}
}

type createUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func CreateAppUser(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req createUserReq
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}
		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" || req.Password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username and password required"})
		}
		role := normalizeRole(req.Role)
		if role == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role must be 'admin' or 'user'"})
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "hash error"})
		}
		res, err := db.Exec(`INSERT INTO app_user (username, password_hash, role) VALUES (?, ?, ?)`, req.Username, string(hash), role)
		if err != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "username taken"})
		}
		id, _ := res.LastInsertId()
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id, "username": req.Username, "role": role})
	}
}

type updateUserReq struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
	Role     *string `json:"role"`
}

func UpdateAppUser(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing id"})
		}
		var req updateUserReq
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
		}

		// Load current
		var curUsername, curRole string
		if err := db.QueryRow(`SELECT username, role FROM app_user WHERE id=?`, id).Scan(&curUsername, &curRole); err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
		}

		newUsername := curUsername
		newRole := curRole
		var newHash string
		var setPassword bool

		if req.Username != nil {
			u := strings.TrimSpace(*req.Username)
			if u == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "username cannot be empty"})
			}
			newUsername = u
		}
		if req.Role != nil {
			r := normalizeRole(*req.Role)
			if r == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role must be 'admin' or 'user'"})
			}
			newRole = r
		}
		if req.Password != nil {
			if strings.TrimSpace(*req.Password) == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password cannot be empty"})
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "hash error"})
			}
			newHash = string(hash)
			setPassword = true
		}

		// Prevent removing last admin if demoting this account
		if strings.ToLower(curRole) == "admin" && strings.ToLower(newRole) != "admin" {
			ok, err := hasAnotherAdmin(db, id)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			if !ok {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot demote the last admin"})
			}
		}

		// Build update
		if setPassword {
			_, err := db.Exec(`UPDATE app_user SET username=?, role=?, password_hash=? WHERE id=?`, newUsername, newRole, newHash, id)
			if err != nil {
				return translateUserWriteErr(c, err)
			}
		} else {
			_, err := db.Exec(`UPDATE app_user SET username=?, role=? WHERE id=?`, newUsername, newRole, id)
			if err != nil {
				return translateUserWriteErr(c, err)
			}
		}
		return c.JSON(fiber.Map{"id": id, "username": newUsername, "role": newRole})
	}
}

func DeleteAppUser(db *sql.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing id"})
		}

		// Load current role
		var role string
		if err := db.QueryRow(`SELECT role FROM app_user WHERE id=?`, id).Scan(&role); err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
		}
		if strings.ToLower(role) == "admin" {
			ok, err := hasAnotherAdmin(db, id)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			if !ok {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot delete the last admin"})
			}
		}

		if _, err := db.Exec(`DELETE FROM app_user WHERE id=?`, id); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		// Sessions cascade via FK
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func normalizeRole(r string) string {
	r = strings.ToLower(strings.TrimSpace(r))
	switch r {
	case "admin", "user":
		return r
	default:
		return ""
	}
}

func hasAnotherAdmin(db *sql.DB, excludeID string) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM app_user WHERE lower(role)='admin' AND id <> ?`, excludeID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func translateUserWriteErr(c fiber.Ctx, err error) error {
	if err == nil {
		return c.JSON(fiber.Map{"ok": true})
	}
	// crude check for uniqueness violations across sqlite/sqlite3
	msg := err.Error()
	if strings.Contains(msg, "UNIQUE") || strings.Contains(msg, "unique") {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "username taken"})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errors.New(msg).Error()})
}
