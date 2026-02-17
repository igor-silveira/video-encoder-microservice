package services

type StorageService interface {
	Download(videoID string, filePath string) error
	Upload(videoPath string, outputDest string, concurrency int, doneUpload chan string) error
}
