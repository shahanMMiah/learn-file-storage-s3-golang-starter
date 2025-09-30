package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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

	const storageSize = 10 << 20
	r.ParseMultipartForm(storageSize)
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusNotFound, "file not found from key thumbnail", err)
		return
	}

	vid, err := cfg.db.GetVideo(videoID)

	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorised user", err)
		return
	}

	mediatype, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))

	if err != nil {
		respondWithError(w, http.StatusNotFound, "Unauthorised user", err)
		return
	}

	if mediatype != "image/jpeg" && mediatype != "image/png" {
		respondWithError(w, http.StatusNotFound, "cannot use file type for thumbnail", err)
		return
	}

	contentType := strings.Split(mediatype, "/")
	extension := contentType[len(contentType)-1]

	randByte := make([]byte, 32)
	rand.Read(randByte)
	randStr := base64.RawURLEncoding.EncodeToString(randByte)

	thumbnailPath := filepath.Join(
		cfg.assetsRoot,
		fmt.Sprintf("%s.%s", randStr, extension))

	thmbFile, err := os.Create(thumbnailPath)

	if err != nil {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("unable to create file %s", thumbnailPath), err)
		return
	}

	_, err = io.Copy(thmbFile, file)
	if err != nil {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("unable to copy thumbnail to %s", thumbnailPath), err)
		return
	}

	thmburl := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, randStr, extension)

	vid.ThumbnailURL = &thmburl
	err = cfg.db.UpdateVideo(vid)
	payload, err := json.Marshal(vid)

	respondWithJSON(w, http.StatusOK, payload)
}
