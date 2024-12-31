package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	mime, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to retrieve asset mime type", err)
		return
	}

	if mime != "image/jpeg" && mime != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type. Please upload a jpeg or png.", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to retrieve video metadata", err)
		return
	}

	thumbnailRand := make([]byte, 32)
	_, err = rand.Read(thumbnailRand)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to get random thumbnail name", err)
		return
	}

	thumbnailName := base64.RawURLEncoding.EncodeToString(thumbnailRand)

	path := filepath.Join(cfg.assetsRoot, thumbnailName+"."+strings.Split(mime, "/")[1])

	asset, err := os.Create(path)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to save new asset", err)
		return
	}

	_, err = io.Copy(asset, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to copy content to new asset", err)
		return
	}

	thumbnailUrl := fmt.Sprintf("http://localhost:%s/%s", cfg.port, path)
	video.ThumbnailURL = &thumbnailUrl

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to save video data", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
