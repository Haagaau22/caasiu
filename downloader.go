package main

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"bytes"

	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
)

type Downloader struct {
	url           string
	concurrencyN  int
	filepath      string
	client        *http.Client
	contentLength int64
	bar           *progressbar.ProgressBar
}


type WriterTask struct {
	rangeStart int
	// buf []byte
	buf bytes.Buffer
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

func (d *Downloader) calcDownloadedSize() ([]int, int) {
	downloadedSizeList := make([]int, d.concurrencyN)
	totalDownloadSize := 0
	for i := 0; i < d.concurrencyN; i++ {
		filepath := fmt.Sprintf("%v-%v", d.filepath, i)

		if fileInfo, err := os.Stat(filepath); err != nil {
			downloadedSizeList[i] = 0
		} else {
			downloadedSizeList[i] = int(fileInfo.Size())
		}
		totalDownloadSize += downloadedSizeList[i]
	}
	return downloadedSizeList, totalDownloadSize
}

func (d *Downloader) Download() {
	log.Printf("url: %v\n", d.url)

	//  request header
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
	d.contentLength = resp.ContentLength

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

	// set progressbar
	d.bar = progressbar.NewOptions(
		int(d.contentLength),
		progressbar.OptionSetWriter(ansi.NewAnsiStdout()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetDescription("downloading..."),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	downloadedSizeList, totalDownloadSize := d.calcDownloadedSize()
	if err := d.bar.Set(totalDownloadSize); err != nil {
		log.Fatal(err)
	}

	// set partSize
	var downloaderWg sync.WaitGroup
	downloaderWg.Add(d.concurrencyN)

	var writerWg sync.WaitGroup
	writerWg.Add(1)
	partSize := int(d.contentLength) / d.concurrencyN

	writerQueue := make(chan WriterTask)
	// download
	for i := 0; i < d.concurrencyN; i++ {
		rangeStart := i*partSize + downloadedSizeList[i]
		rangeEnd := i*partSize + partSize - 1
		if i == d.concurrencyN-1 {
			rangeEnd = int(d.contentLength)
		}

		go httpDownload(d.client, d.url, rangeStart, rangeEnd, writerQueue, &downloaderWg, d.bar)
	}

	go merge(writerQueue, d.filepath, &writerWg)
	downloaderWg.Wait()
	close(writerQueue)
	log.Println("close writerQueue")

	writerWg.Wait()
	log.Println("finished!")

}



func httpDownload(
	client *http.Client, 
	url string, 
	rangeStart, rangeEnd int, 
	writerQueue chan WriterTask,
	wg *sync.WaitGroup, 
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

	copyBuf := make([]byte, 128*1024)
	if _, err := io.CopyBuffer(io.MultiWriter(&buf, bar), resp.Body, copyBuf); err != nil && err != io.EOF {
		log.Fatal(err)
	}


	writerQueue <- WriterTask{buf: buf, rangeStart: rangeStart}
}



func merge(writerQueue chan WriterTask, filepath string, wg *sync.WaitGroup) {
	defer wg.Done()
	file, err := os.Create(filepath)
	if err != nil {
		log.Fatal(err)
	}

	for task := range writerQueue {
		// log.Println("merging\t", task.rangeStart)

		if _, err := file.WriteAt(task.buf.Bytes(), int64(task.rangeStart)); err != nil {
			log.Fatal(err)
		}

	}
}
