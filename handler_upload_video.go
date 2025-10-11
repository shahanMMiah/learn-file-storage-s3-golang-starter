package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"mime"
	"net/http"
	"os"
	"os/exec"
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
	http.MaxBytesReader(w, r.Body, storageSize)

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

	_, err = io.Copy(tempfile, file)
	if err != nil {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("unable to copy thumbnail to %s", tempfile.Name()), err)
		return
	}

	tempfile.Seek(0, io.SeekStart)

	var ratioKey string
	aspectRatio, err := GetVideoAspectRatio(tempfile.Name())
	switch aspectRatio {
	case "16:9":
		ratioKey = "landscape"

	case "9:16":
		ratioKey = "portrait"

	default:
		ratioKey = "other"
	}

	randByte := make([]byte, 32)
	rand.Read(randByte)
	randStr := base64.RawURLEncoding.EncodeToString(randByte)

	videoKey := fmt.Sprintf("%s/%s.%s", ratioKey, randStr, extension)

	procStr, err := proccessVideoForFastStart(tempfile.Name())

	if err != nil {
		respondWithError(w, http.StatusFailedDependency, fmt.Sprintf("%s", err.Error()), err)
		return
	}

	procfile, err := os.Open(procStr)
	if err != nil {
		respondWithError(w, http.StatusFailedDependency, fmt.Sprintf("%s", err.Error()), err)
		return
	}
	defer procfile.Close()

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{Bucket: &cfg.s3Bucket, Key: &videoKey, Body: procfile, ContentType: &mediatype})
	if err != nil {
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("%s", err.Error()), err)
		return
	}

	vidurl := fmt.Sprintf("%s/%s", cfg.s3CfDistribution, videoKey)
	//vidurl := fmt.Sprintf("%s,%s", cfg.s3Bucket, videoKey)

	vid.VideoURL = &vidurl
	err = cfg.db.UpdateVideo(vid)
	payload, err := json.Marshal(vid)
	respondWithJSON(w, http.StatusOK, payload)

}

func GetVideoAspectRatio(filePath string) (string, error) {

	probeCmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	probeCmd.Stdout = bytes.NewBuffer(make([]byte, 0))

	err := probeCmd.Run()
	if err != nil {
		return "", err
	}

	type streams struct {
		Width  int64 `json:"width"`
		Height int64 `json:"height"`
	}

	aspectData := struct {
		Streams []streams `json:"streams"`
	}{}

	bytesOut, ok := probeCmd.Stdout.(*bytes.Buffer)
	if !ok {
		return "", fmt.Errorf("stdOut is not type *bytesBuffer")
	}
	json.Unmarshal(bytesOut.Bytes(), &aspectData)

	width := aspectData.Streams[0].Width
	height := aspectData.Streams[0].Height

	Gdc := big.NewInt(0).GCD(nil, nil, big.NewInt(width), big.NewInt(height))

	fmt.Printf("GDC is %v and width %v - height %v\n", Gdc, width, height)

	widthRatio := width / Gdc.Int64()
	heightRatio := height / Gdc.Int64()

	ratio := fmt.Sprintf("%d:%d", widthRatio, heightRatio)

	switch ratio {
	case "16:9":
		return ratio, nil

	case "9:16":
		return ratio, nil

	default:
		return "other", nil
	}
}

func proccessVideoForFastStart(filepath string) (string, error) {
	procString := fmt.Sprintf("%s.proccessing.mp4", filepath)
	procCmd := exec.Command("ffmpeg", "-i", filepath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", procString)

	err := procCmd.Run()
	if err != nil {
		return "", err
	}

	return procString, nil
}
