package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

type stream struct {
	Streams []struct {
		Width  int `json:"width,omitempty"`
		Height int `json:"height,omitempty"`
	} `json:"streams"`
}

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	var b bytes.Buffer
	var e bytes.Buffer

	cmd.Stdout = &b
	cmd.Stderr = &e

	err := cmd.Run()
	if err != nil {
		return "", errors.New(e.String())
	}

	res := stream{}
	err = json.Unmarshal(b.Bytes(), &res)
	if err != nil {
		return "", err
	}

	var sixteenByNine, nineBySixteen float32
	sixteenByNine = 16 / 9
	nineBySixteen = 9 / 16

	if float32(res.Streams[0].Width/res.Streams[0].Height) == sixteenByNine {
		return "16:9", nil
	} else if float32(res.Streams[0].Width/res.Streams[0].Height) == nineBySixteen {
		return "9:16", nil
	}

	return "other", nil
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"

	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)

	var b bytes.Buffer
	var e bytes.Buffer

	cmd.Stdout = &b
	cmd.Stderr = &e

	err := cmd.Run()
	if err != nil {
		fmt.Println(e.String())
		return "", errors.New(e.String())
	}

	return outputFilePath, nil
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)

	params := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	req, err := presignClient.PresignGetObject(context.Background(), &params, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}
	vals := strings.Split(*video.VideoURL, ", ")
	bucket := strings.Trim(vals[0], " ")
	key := strings.Trim(vals[1], " ")

	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, time.Minute*15)
	if err != nil {
		return video, err
	}

	video.VideoURL = &presignedURL

	return video, nil
}
