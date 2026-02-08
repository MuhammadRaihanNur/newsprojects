package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Post struct {
	ID        int64     `json:"id"`
	Caption   string    `json:"caption"`
	ImageURL  string    `json:"imageUrl"`
	CreatedAt time.Time `json:"createdAt"`
}

type Server struct {
	db *sql.DB
}

func main() {
	// Pastikan folder uploads ada
	if err := os.MkdirAll("uploads", 0755); err != nil {
		log.Fatal(err)
	}

	// Ambil DSN dari env (lebih aman)
	// contoh DSN:
	// export MYSQL_DSN="user:pass@tcp(127.0.0.1:3306)/news_project?parseTime=true&charset=utf8mb4&loc=Asia%2FJakarta"
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		// fallback (boleh, tapi mending pakai env)
		dsn = "root:root@tcp(127.0.0.1:3306)/news_project?parseTime=true&charset=utf8mb4&loc=Asia%2FJakarta"
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("sql.Open:", err)
	}
	defer db.Close()

	// Test koneksi
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal("db.Ping:", err)
	}

	s := &Server{db: db}

	mux := http.NewServeMux()

	// Static FE
	mux.Handle("/", http.FileServer(http.Dir("./public")))

	// Serve uploaded images
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	// API
	mux.HandleFunc("/api/posts", s.handlePosts)
	mux.HandleFunc("/api/posts/", s.handlePostByID) // note: trailing slash

	addr := ":8080"
	log.Println("Server running on http://localhost" + addr)
	log.Fatal(http.ListenAndServe(addr, withCORS(mux)))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handlePosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetPosts(w, r)
	case http.MethodPost:
		s.handleCreatePost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleGetPosts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, image_url, caption, created_at
		FROM posts
		ORDER BY id DESC
		LIMIT 100
	`)
	if err != nil {
		http.Error(w, "DB query error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.ImageURL, &p.Caption, &p.CreatedAt); err != nil {
			http.Error(w, "DB scan error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		posts = append(posts, p)
	}

	writeJSON(w, posts)
}

func (s *Server) handleCreatePost(w http.ResponseWriter, r *http.Request) {
	// limit 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	caption := strings.TrimSpace(r.FormValue("caption"))
	if caption == "" {
		http.Error(w, "Caption is required", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Image is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !isAllowedImage(header) {
		http.Error(w, "Only JPG, PNG, WEBP images are allowed", http.StatusBadRequest)
		return
	}

	filename, err := saveUpload(file, header)
	if err != nil {
		http.Error(w, "Failed to save image: "+err.Error(), http.StatusInternalServerError)
		return
	}

	imageURL := "/uploads/" + filename

	// Insert ke MySQL
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO posts (image_url, caption) VALUES (?, ?)`,
		imageURL, caption,
	)
	if err != nil {
		http.Error(w, "DB insert error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	id, _ := res.LastInsertId()

	// Ambil created_at yang benar dari DB (biar konsisten)
	var createdAt time.Time
	err = s.db.QueryRowContext(ctx, `SELECT created_at FROM posts WHERE id = ?`, id).Scan(&createdAt)
	if err != nil {
		createdAt = time.Now()
	}

	post := Post{
		ID:        id,
		Caption:   caption,
		ImageURL:  imageURL,
		CreatedAt: createdAt,
	}
	writeJSON(w, post)
}

func isAllowedImage(h *multipart.FileHeader) bool {
	ext := strings.ToLower(filepath.Ext(h.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	default:
		return false
	}
}

func saveUpload(file multipart.File, h *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(h.Filename))
	name := strconv.FormatInt(time.Now().UnixNano(), 10) + ext
	dstPath := filepath.Join("uploads", name)

	dst, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}
	return name, nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// halaman detail
func (s *Server) handlePostByID(w http.ResponseWriter, r *http.Request) {
	// format: /api/posts/{id}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/posts/")
	idStr = strings.TrimSpace(idStr)
	if idStr == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var p Post
	err = s.db.QueryRowContext(ctx, `
		SELECT id, image_url, caption, created_at
		FROM posts
		WHERE id = ?
	`, id).Scan(&p.ID, &p.ImageURL, &p.Caption, &p.CreatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, p)
}

// halaman edit
