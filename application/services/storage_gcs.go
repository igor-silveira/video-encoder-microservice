package services

import (
	"cloud.google.com/go/storage"
	"context"
	"io/ioutil"
	"log"
	"os"
	"runtime"
)

type GCSStorageService struct{}

func NewGCSStorageService() *GCSStorageService {
	return &GCSStorageService{}
}

func (g *GCSStorageService) Download(videoID string, filePath string) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}

	bkt := client.Bucket(os.Getenv("INPUT_BUCKET_NAME"))
	obj := bkt.Object(filePath)
	r, err := obj.NewReader(ctx)
	if err != nil {
		return err
	}
	defer r.Close()

	body, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	f, err := os.Create(os.Getenv("LOCAL_STORAGE_PATH") + "/" + videoID + ".mp4")
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(body)
	if err != nil {
		return err
	}

	log.Printf("video %v has been stored", videoID)
	return nil
}

func (g *GCSStorageService) Upload(videoPath string, outputDest string, concurrency int, doneUpload chan string) error {
	videoUpload := NewVideoUpload()
	videoUpload.OutputBucket = outputDest
	videoUpload.VideoPath = videoPath

	if concurrency == 0 {
		concurrency = runtime.NumCPU()
	}

	go videoUpload.ProcessUpload(concurrency, doneUpload)

	return nil
}
