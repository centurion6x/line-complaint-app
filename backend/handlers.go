package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
)

type Server struct {
	cfg    *Config
	line   *LineClient
	images *ImageStore
}

func NewServer(cfg *Config, line *LineClient, images *ImageStore) *Server {
	return &Server{cfg: cfg, line: line, images: images}
}

type complaintResponse struct {
	OK       bool   `json:"ok"`
	TicketID string `json:"ticketId,omitempty"`
	Error    string `json:"error,omitempty"`
}

// handleCreateComplaint is the only state-changing endpoint in this
// service. It: verifies the caller is a real, current LINE session
// (via the LIFF ID token), accepts a title/description/photos,
// caches the photos in memory, and pushes a Flex acknowledgement plus
// the images back to that user's chat. Nothing is written to a
// database — the "record" of the complaint is the chat message itself.
func (s *Server) handleCreateComplaint(w http.ResponseWriter, r *http.Request) {
	// Generous cap: a handful of photos at MaxUploadMB each, plus headroom
	// for form fields and multipart overhead.
	maxBytes := (s.cfg.MaxUploadMB*int64(s.cfg.MaxPhotos) + 2) * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, complaintResponse{Error: "request too large or malformed"})
		return
	}

	idToken := r.FormValue("idToken")
	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))

	if idToken == "" {
		writeJSON(w, http.StatusUnauthorized, complaintResponse{Error: "missing LINE session token — open this page from inside LINE"})
		return
	}
	if title == "" {
		writeJSON(w, http.StatusBadRequest, complaintResponse{Error: "title is required"})
		return
	}
	if len(title) > 200 {
		writeJSON(w, http.StatusBadRequest, complaintResponse{Error: "title must be under 200 characters"})
		return
	}
	if len(description) > 2000 {
		writeJSON(w, http.StatusBadRequest, complaintResponse{Error: "description must be under 2000 characters"})
		return
	}

	userID, err := s.line.VerifyIDToken(idToken)
	if err != nil {
		log.Printf("id token verification failed: %v", err)
		writeJSON(w, http.StatusUnauthorized, complaintResponse{Error: "could not verify your LINE session — close and reopen this page from LINE"})
		return
	}

	files := r.MultipartForm.File["photos"]
	if len(files) > s.cfg.MaxPhotos {
		writeJSON(w, http.StatusBadRequest, complaintResponse{Error: fmt.Sprintf("at most %d photos are allowed", s.cfg.MaxPhotos)})
		return
	}

	imageURLs, errResp := s.cachePhotos(files)
	if errResp != "" {
		writeJSON(w, http.StatusBadRequest, complaintResponse{Error: errResp})
		return
	}

	ticketID := shortTicketID()

	msgs := []message{flexAck(ticketID, title, description, len(imageURLs))}
	for _, u := range imageURLs {
		// The Messaging API requires both an original and a preview URL;
		// we point both at the same cached copy to keep this simple.
		msgs = append(msgs, imageMessage(u, u))
	}

	if err := s.line.Push(userID, msgs); err != nil {
		log.Printf("push to %s failed: %v", userID, err)
		writeJSON(w, http.StatusBadGateway, complaintResponse{Error: "your complaint was captured but the chat acknowledgement failed to send — please check LINE in a moment"})
		return
	}

	writeJSON(w, http.StatusOK, complaintResponse{OK: true, TicketID: ticketID})
}

func (s *Server) cachePhotos(files []*multipart.FileHeader) ([]string, string) {
	var imageURLs []string
	for _, fh := range files {
		if fh.Size > s.cfg.MaxUploadMB*1024*1024 {
			return nil, fmt.Sprintf("each photo must be under %dMB", s.cfg.MaxUploadMB)
		}
		f, err := fh.Open()
		if err != nil {
			return nil, "could not read an uploaded photo"
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, "could not read an uploaded photo"
		}

		contentType := fh.Header.Get("Content-Type")
		if contentType != "image/jpeg" && contentType != "image/png" {
			contentType = http.DetectContentType(data)
		}
		if contentType != "image/jpeg" && contentType != "image/png" {
			return nil, "only JPEG or PNG photos are accepted (the Messaging API's image message requirement)"
		}

		ext := "jpg"
		if contentType == "image/png" {
			ext = "png"
		}
		id := s.images.Put(data, contentType)
		imageURLs = append(imageURLs, fmt.Sprintf("%s/images/%s.%s", s.cfg.PublicBaseURL, id, ext))
	}
	return imageURLs, ""
}

func (s *Server) handleGetImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if i := strings.LastIndex(id, "."); i != -1 {
		id = id[:i]
	}
	data, contentType, ok := s.images.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.Write(data)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func shortTicketID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "C-" + strings.ToUpper(hex.EncodeToString(b))
}
