package main

import (
	"encoding/json"
	"fmt"
	"io"
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

	contentType := strings.Split(header.Header.Get("Content-Type"), "/")
	extension := contentType[len(contentType)-1]

	thumbnailPath := filepath.Join(
		cfg.assetsRoot,
		fmt.Sprintf("%s.%s", videoID.String(), extension))

	thmbFile, err := os.Create(thumbnailPath)

	if err != nil {
		respondWithError(w, http.StatusCreated, fmt.Sprintf("unable to create file %s", thumbnailPath), err)
		return
	}

	_, err = io.Copy(thmbFile, file)
	if err != nil {
		respondWithError(w, http.StatusCreated, fmt.Sprintf("unable to copy thumbnail to %s", thumbnailPath), err)
		return
	}

	thmburl := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, videoID, extension)

	vid.ThumbnailURL = &thmburl
	err = cfg.db.UpdateVideo(vid)
	payload, err := json.Marshal(vid)

	respondWithJSON(w, http.StatusOK, payload)
}
