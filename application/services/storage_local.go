package services

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

type LocalStorageService struct{}

func NewLocalStorageService() *LocalStorageService {
	return &LocalStorageService{}
}

func (l *LocalStorageService) Download(videoID string, filePath string) error {
	inputPath := os.Getenv("INPUT_LOCAL_PATH") + "/" + filePath
	outputPath := os.Getenv("LOCAL_STORAGE_PATH") + "/" + videoID + ".mp4"

	src, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	log.Printf("video %v has been stored (local)", videoID)
	return nil
}

func (l *LocalStorageService) Upload(videoPath string, outputDest string, concurrency int, doneUpload chan string) error {
	go func() {
		err := copyDir(videoPath, outputDest)
		if err != nil {
			doneUpload <- err.Error()
			return
		}
		doneUpload <- "upload completed"
	}()

	return nil
}

func copyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, os.ModePerm)
		}

		return copyFile(path, targetPath)
	})
}

func copyFile(src string, dst string) error {
	err := os.MkdirAll(filepath.Dir(dst), os.ModePerm)
	if err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
