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

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {

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

	const storageSize = 1 << 30
	clampBody := http.MaxBytesReader(w, r.Body, storageSize)

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	vid, err := cfg.db.GetVideo(videoID)

	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorised user", err)
		return
	}
	if vid.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorised user", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusNotFound, "file not found from key thumbnail", err)
		return
	}
	defer file.Close()

	mediatype, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))

	if err != nil {
		respondWithError(w, http.StatusNotFound, "Unauthorised user", err)
		return
	}

	if mediatype != "video/mp4" {
		respondWithError(w, http.StatusNotFound, "filetype not mp4", err)
		return
	}

	contentType := strings.Split(mediatype, "/")
	extension := contentType[len(contentType)-1]

	tempfile, err := os.CreateTemp("", "tubely-upload.mp4")
	defer os.Remove(tempfile.Name())
	defer tempfile.Close()

	_, err = io.Copy(tempfile, clampBody)
	if err != nil {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("unable to copy thumbnail to %s", tempfile.Name()), err)
		return
	}

	tempfile.Seek(0, io.SeekStart)

	randByte := make([]byte, 32)
	rand.Read(randByte)
	randStr := base64.RawURLEncoding.EncodeToString(randByte)

	videoKey := filepath.Join(
		fmt.Sprintf("%s.%s", randStr, extension))

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{Bucket: &cfg.s3Bucket, Key: &videoKey, Body: tempfile, ContentType: &mediatype})
	if err != nil {
		respondWithError(w, http.StatusNotFound, "unable to store file to bucket", err)
		return
	}

	vidurl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, videoKey)

	vid.VideoURL = &vidurl
	err = cfg.db.UpdateVideo(vid)
	payload, err := json.Marshal(vid)
	respondWithJSON(w, http.StatusOK, payload)

}
