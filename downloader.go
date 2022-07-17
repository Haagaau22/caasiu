package main

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"sync"

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

func (d *Downloader) generateFilepath(inputFilepath, headerFilename string) string {
	dirpath := ""
	isDir := false
	if file, err := os.Stat(inputFilepath); err == nil && file.IsDir() {
		dirpath = inputFilepath
		isDir = true
	}

	filename := path.Base(d.url)
	if headerFilename != "" {
		filename = headerFilename
	}
	if !isDir && inputFilepath != "" {
		filename = inputFilepath
	}
	return path.Join(dirpath, filename)
}

func (d *Downloader) calcDownloadedSizeList() []int {
	downloadedSizeList := make([]int, d.concurrencyN)
	for i := 0; i < d.concurrencyN; i++ {
		filepath := fmt.Sprintf("%v-%v", d.filepath, i)

		if fileInfo, err := os.Stat(filepath); err != nil {
			downloadedSizeList[i] = 0
		} else {
			downloadedSizeList[i] = int(fileInfo.Size())
		}
	}
	return downloadedSizeList
}

func (d *Downloader) Download() {

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

	downloadedSizeList := d.calcDownloadedSizeList()
	log.Println(downloadedSizeList)
	for _, downloadSize := range downloadedSizeList {
		d.bar.Add(downloadSize)
	}
	log.Printf("url: %v\nfilepath: %v\nconcurrencyN: %v", d.url, d.filepath, d.concurrencyN)

	// set partSize
	var wg sync.WaitGroup
	wg.Add(d.concurrencyN)
	partSize := int(d.contentLength) / d.concurrencyN

	// download
	for i := 0; i < d.concurrencyN; i++ {
		rangeStart := i*partSize + downloadedSizeList[i]
		rangeEnd := rangeStart + partSize - 1
		if i == d.concurrencyN-1 {
			rangeEnd = int(d.contentLength)
		}
		filepath := fmt.Sprintf("%v-%v", d.filepath, i)

		go func() {
			defer wg.Done()

			downloadReq, err := http.NewRequest(http.MethodGet, d.url, nil)
			if err != nil {
				log.Fatal(err)
			}
			downloadReq.Header.Set("Range", fmt.Sprintf("bytes=%v-%v", rangeStart, rangeEnd))

			d.download(downloadReq, filepath)

		}()
	}
	wg.Wait()
	d.merge()

}

func (d *Downloader) download(req *http.Request, filepath string) {
	// log.Printf("start downloading, range: %v, filepath: %v\n", req.Header.Get("Range"), filepath)
	resp, err := d.client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	buf := make([]byte, 128*1024)
	if _, err := io.CopyBuffer(io.MultiWriter(file, d.bar), resp.Body, buf); err != nil && err != io.EOF {
		log.Fatal(err)
	}
	// log.Println("finish downloading", filepath)

}

func (d *Downloader) merge() {
	dstFile, err := os.OpenFile(d.filepath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer dstFile.Close()

	for i := 0; i < d.concurrencyN; i++ {
		srcFilepath := fmt.Sprintf("%v-%v", d.filepath, i)
		srcFile, err := os.Open(srcFilepath)
		if err != nil {
			log.Fatal(err)
		}

		buf := make([]byte, 128*1024)
		if _, err := io.CopyBuffer(dstFile, srcFile, buf); err != nil && err != io.EOF {
			log.Fatal(err)
		}
		srcFile.Close()

		os.Remove(srcFilepath)

	}
}
