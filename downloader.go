package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
)

type Downloader struct {
	url           string
	blockSize     int
	concurrencyN  int
	filepath      string
	client        *http.Client
	contentLength int
	bar           *progressbar.ProgressBar
}

type Task struct {
	rangeStart int
	rangeEnd int
}

func (d *Downloader) generateFilepath(inputFilepath, headerFilename string) string {
	dirpath := ""
	isDir := false
	if file, err := os.Stat(inputFilepath); err == nil && file.IsDir() {
		dirpath = inputFilepath
		isDir = true
	}

	filename := strings.Split(path.Base(d.url), "?")[0]
	if headerFilename != "" {
		filename = headerFilename
	}
	if !isDir && inputFilepath != "" {
		filename = inputFilepath
	}
	return path.Join(dirpath, filename)
}


func checkBlock(file *os.File, rangeStart, rangeEnd int) bool {

	if file == nil {
		return false
	}

	bufLen := 32
	if (rangeEnd - rangeStart) / 10 < bufLen {
		bufLen = int((rangeEnd - rangeStart) / 10)
	} 
	buf := make([]byte, bufLen)

	if _, err := file.ReadAt(buf, int64(rangeEnd - bufLen)); errors.Is(err, io.EOF) {
		return false
	}else if  err != nil {
		log.Fatal(err)
	}
	for i := bufLen-1; i>0; i--{
		if buf[i] != byte(0){
			return true
		}
	}
	return false
}


func generateTasks(
	filepath string, 
	blockSize, maxSize int, 
	bar *progressbar.ProgressBar) []Task{

	var file *os.File

	if _, err := os.Stat(filepath); err == nil {
		file, err = os.Open(filepath)
		log.Printf("load old file")
		if err != nil {
			log.Fatal(err)
		}
	}



	var taskList []Task

	rangeStart := 0
	rangeEnd := blockSize - 1
	for rangeStart < maxSize - 1 {
		if rangeEnd >= maxSize - 1 {
			rangeEnd = maxSize - 1
		}

		hasDownloadBlock := checkBlock(file, rangeStart, rangeEnd)
		log.Printf(
			"block, hasDownload: %v, rangeStart: %d, rangeEnd: %d", 
			hasDownloadBlock, rangeStart, rangeEnd)

		if !hasDownloadBlock {
			taskList = append(taskList, Task{rangeStart:rangeStart, rangeEnd:rangeEnd})
		}else if err := bar.Add(rangeEnd-rangeStart); err != nil {
			log.Fatal(err)
		} 

		rangeStart = rangeEnd + 1
		rangeEnd = rangeStart + blockSize - 1
	}
	return taskList
}

func (d *Downloader) Download() {
	log.Printf("url: %v\n", d.url)

	//  send heading
	req, err := http.NewRequest(http.MethodHead, d.url, nil)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	log.Println("finish heading")

	// set the max size
	d.contentLength = int(resp.ContentLength)

	// set the number of concurrency
	acceptRanges := resp.Header.Get("Accept-Ranges")
	if acceptRanges != "bytes" {
		d.concurrencyN = 1
		log.Printf("%v, partial request is not supported, reset concurrencyN=1", d.url)
	}

	// set filepath
	headerFilename := ""
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
		headerFilename = params["filename"]
	}
	d.filepath = d.generateFilepath(d.filepath, headerFilename)

	log.Printf("filepath: %s", d.filepath)

	// set progressbar
	d.bar = progressbar.NewOptions(
		d.contentLength,
		progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetDescription(path.Base(d.filepath)),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	var wg sync.WaitGroup


	concurrencyController := make(chan struct{}, d.concurrencyN)

	taskList := generateTasks(d.filepath, d.blockSize, d.contentLength, d.bar)

	file, err := os.OpenFile(d.filepath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	for _, task := range taskList {
		wg.Add(1)
		concurrencyController <- struct{}{}
		log.Printf("add task: %v -> %v", task.rangeStart, task.rangeEnd)

		go httpDownload(
			d.client, 
			d.url, 
			task.rangeStart, 
			task.rangeEnd, 
			concurrencyController,
			&wg, 
			file, 
			d.bar)
	}


	wg.Wait()
	log.Println("finished!")
}

func httpDownload(
	client *http.Client,
	url string,
	rangeStart, rangeEnd int,
	concurrencyController chan struct{},
	wg *sync.WaitGroup,
	file *os.File,
	bar *progressbar.ProgressBar) {
	log.Printf("start downloading, rangeStart: %d, rangeEnd: %d\n", rangeStart, rangeEnd)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%v-%v", rangeStart, rangeEnd))

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	defer wg.Done()

	var buf bytes.Buffer

	if _, err := io.Copy(io.MultiWriter(&buf, bar), resp.Body); err != nil && err != io.EOF {
		log.Fatal(err)
	}

	if _, err := file.WriteAt(buf.Bytes(), int64(rangeStart)); err != nil {
		log.Fatal(err)
	}
	<- concurrencyController
	log.Printf("finish downloaded block, rangeStart: %d, rangeEnd: %d", rangeStart, rangeEnd)

}
