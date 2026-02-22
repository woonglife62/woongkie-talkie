package handler

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/woonglife62/woongkie-talkie/pkg/logger"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const maxUploadSize = 10 << 20 // 10MB

var allowedMimeTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
	"application/pdf": true,
	"text/plain":      true,
}

// mimeToExtensions maps allowed MIME types to their valid file extensions.
var mimeToExtensions = map[string][]string{
	"image/jpeg":      {".jpg", ".jpeg"},
	"image/png":       {".png"},
	"image/gif":       {".gif"},
	"image/webp":      {".webp"},
	"application/pdf": {".pdf"},
	"text/plain":      {".txt"},
}

// sanitizeFilename removes characters that could be used for header injection or path traversal.
var unsafeFilenameChars = regexp.MustCompile(`[^\w.\-]`)

func sanitizeFilename(name string) string {
	base := filepath.Base(name)
	return unsafeFilenameChars.ReplaceAllString(base, "_")
}

func extMatchesMime(ext, mime string) bool {
	if exts, ok := mimeToExtensions[mime]; ok {
		for _, e := range exts {
			if e == ext {
				return true
			}
		}
	}
	return false
}

// isPrivateIP checks if an IP is a private/loopback address (SSRF defence).
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",
		"::1/128",
	}
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// POST /rooms/:id/upload
// #244: use absolute path for uploadDir
// #243: sanitize filename to prevent Content-Disposition injection
// #238/#272: SSRF defence via net.LookupHost + private IP check (IPv4 and IPv6)
// #176: path traversal prevention
// #206: save file before DB insert; delete file if DB insert fails
// #178: extension-based MIME cross-check (already present via extMatchesMime)
func UploadFileHandler(c echo.Context) error {
	if err := requireRoomMember(c); err != nil {
		return err
	}
	roomID := c.Param("id")
	username := GetUsername(c)
	if username == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "인증이 필요합니다"})
	}

	// #238/#272: SSRF – resolve hostname and reject private/loopback IPs
	host := c.Request().Host
	if host != "" {
		hostOnly := host
		if h, _, err := net.SplitHostPort(host); err == nil {
			hostOnly = h
		}
		if ip := net.ParseIP(hostOnly); ip != nil {
			if isPrivateIP(ip) {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "허용되지 않는 호스트입니다"})
			}
		} else {
			addrs, err := net.LookupHost(hostOnly)
			if err == nil {
				for _, addr := range addrs {
					if ip := net.ParseIP(addr); ip != nil && isPrivateIP(ip) {
						return c.JSON(http.StatusForbidden, map[string]string{"error": "허용되지 않는 호스트입니다"})
					}
				}
			}
		}
	}

	// Limit request body size.
	c.Request().Body = http.MaxBytesReader(c.Response().Writer, c.Request().Body, maxUploadSize)

	file, header, err := c.Request().FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "파일을 읽을 수 없습니다"})
	}
	defer file.Close()

	// Read first 512 bytes for MIME detection.
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "파일을 읽을 수 없습니다"})
	}
	detectedMime := http.DetectContentType(buf[:n])

	if !allowedMimeTypes[detectedMime] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "허용되지 않는 파일 형식입니다"})
	}

	// #243: sanitize original filename
	originalName := sanitizeFilename(header.Filename)
	ext := strings.ToLower(filepath.Ext(originalName))

	// #176: prevent path traversal via extension
	if strings.Contains(ext, "..") || strings.Contains(originalName, "..") {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "유효하지 않은 파일 이름입니다"})
	}

	// #178: extension must match detected MIME
	if !extMatchesMime(ext, detectedMime) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "파일 확장자가 내용과 일치하지 않습니다"})
	}

	uniqueName := fmt.Sprintf("%s%s", primitive.NewObjectID().Hex(), ext)

	// #244: use absolute path for upload directory
	relUploadDir := filepath.Join("uploads", filepath.Clean(roomID))
	absUploadDir, err := filepath.Abs(relUploadDir)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "경로 처리에 실패했습니다"})
	}

	// #176: ensure roomID doesn't escape uploads directory
	absUploadsBase, _ := filepath.Abs("uploads")
	if !strings.HasPrefix(absUploadDir, absUploadsBase) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "유효하지 않은 방 ID입니다"})
	}

	if err := os.MkdirAll(absUploadDir, 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "디렉토리 생성에 실패했습니다"})
	}

	savePath := filepath.Join(absUploadDir, uniqueName)

	dst, err := os.Create(savePath)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "파일 저장에 실패했습니다"})
	}
	defer dst.Close()

	// Seek back to start since we already read 512 bytes.
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	} else {
		// Write already-read bytes first.
		if _, err := dst.Write(buf[:n]); err != nil {
			os.Remove(savePath)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "파일 저장에 실패했습니다"})
		}
	}

	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(savePath)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "파일 저장에 실패했습니다"})
	}
	dst.Close()

	fileID := primitive.NewObjectID()
	meta := mongodb.FileMetadata{
		ID:       fileID,
		Filename: originalName,
		MimeType: detectedMime,
		Size:     header.Size,
		Path:     savePath,
		URL:      fmt.Sprintf("/files/%s", fileID.Hex()),
		RoomID:   roomID,
		Uploader: username,
	}

	// #206: if DB insert fails, remove the saved file to avoid orphaned files
	saved, err := mongodb.InsertFileMetadata(meta)
	if err != nil {
		os.Remove(savePath)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "메타데이터 저장에 실패했습니다"})
	}

	// Broadcast MSG_FILE event to the room (#239).
	hub := RoomMgr.GetHub(roomID)
	if hub != nil {
		select {
		case hub.Broadcast <- mongodb.ChatMessage{
			Event:   "MSG_FILE",
			User:    username,
			Message: saved.URL,
			RoomID:  roomID,
		}:
		case <-hub.stop:
		case <-time.After(5 * time.Second):
			logger.Logger.Warnw("UploadFile: broadcast timed out", "room_id", roomID)
		}
	}

	return c.JSON(http.StatusCreated, saved)
}

// GET /files/:fileId
// #261: authentication check is handled by JWT middleware at router level (route is protected)
func ServeFileHandler(c echo.Context) error {
	fileID := c.Param("fileId")

	meta, err := mongodb.FindFileByID(fileID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "파일을 찾을 수 없습니다"})
	}

	// Security: ensure the file path is within the uploads directory.
	absUploadsBase, _ := filepath.Abs("uploads")
	cleanPath := filepath.Clean(meta.Path)
	if !strings.HasPrefix(cleanPath, absUploadsBase) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "접근이 거부되었습니다"})
	}

	c.Response().Header().Set("Content-Type", meta.MimeType)

	// For non-images, force download with sanitized filename.
	if meta.MimeType != "image/jpeg" && meta.MimeType != "image/png" &&
		meta.MimeType != "image/gif" && meta.MimeType != "image/webp" {
		// #243: sanitize filename in Content-Disposition header
		safeFilename := sanitizeFilename(meta.Filename)
		c.Response().Header().Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename="%s"`, safeFilename))
	}

	http.ServeFile(c.Response().Writer, c.Request(), cleanPath)
	return nil
}
