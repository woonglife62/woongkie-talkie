package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	mongodb "github.com/woonglife62/woongkie-talkie/pkg/mongoDB"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const maxUploadSize = 10 << 20 // 10MB

var allowedMimeTypes = map[string]bool{
	"image/jpeg":       true,
	"image/png":        true,
	"image/gif":        true,
	"image/webp":       true,
	"application/pdf":  true,
	"text/plain":       true,
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

// POST /rooms/:id/upload
func UploadFileHandler(c echo.Context) error {
	if err := requireRoomMember(c); err != nil {
		return err
	}
	roomID := c.Param("id")
	username := GetUsername(c)
	if username == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "인증이 필요합니다"})
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

	originalName := filepath.Base(header.Filename)
	ext := filepath.Ext(originalName)
	if !extMatchesMime(ext, detectedMime) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "파일 확장자가 내용과 일치하지 않습니다"})
	}

	uniqueName := fmt.Sprintf("%s%s", primitive.NewObjectID().Hex(), ext)

	// Ensure room upload directory exists.
	uploadDir := filepath.Clean(filepath.Join("uploads", roomID))
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "디렉토리 생성에 실패했습니다"})
	}

	savePath := filepath.Join(uploadDir, uniqueName)

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
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "파일 저장에 실패했습니다"})
		}
	}

	if _, err := io.Copy(dst, file); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "파일 저장에 실패했습니다"})
	}

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

	saved, err := mongodb.InsertFileMetadata(meta)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "메타데이터 저장에 실패했습니다"})
	}

	// Broadcast MSG_FILE event to the room.
	hub := RoomMgr.GetHub(roomID)
	if hub != nil {
		hub.Broadcast <- mongodb.ChatMessage{
			Event:   "MSG_FILE",
			User:    username,
			Message: saved.URL,
			RoomID:  roomID,
		}
	}

	return c.JSON(http.StatusCreated, saved)
}

// GET /files/:fileId
func ServeFileHandler(c echo.Context) error {
	fileID := c.Param("fileId")

	meta, err := mongodb.FindFileByID(fileID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "파일을 찾을 수 없습니다"})
	}

	// Security: ensure the file path is within the uploads directory.
	cleanPath := filepath.Clean(meta.Path)
	uploadsDir := filepath.Clean("uploads")
	if len(cleanPath) <= len(uploadsDir) || cleanPath[:len(uploadsDir)] != uploadsDir {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "접근이 거부되었습니다"})
	}

	c.Response().Header().Set("Content-Type", meta.MimeType)

	// For non-images, force download.
	if meta.MimeType != "image/jpeg" && meta.MimeType != "image/png" &&
		meta.MimeType != "image/gif" && meta.MimeType != "image/webp" {
		c.Response().Header().Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename="%s"`, meta.Filename))
	}

	http.ServeFile(c.Response().Writer, c.Request(), cleanPath)
	return nil
}
