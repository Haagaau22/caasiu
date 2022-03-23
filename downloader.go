package main

import (
	"io"
	"os"
	"log"
	"fmt"
	"sync"
	"net/http"
	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
)

type Downloader struct {
	url string
	concurrencyN int
	filename string
	client *http.Client
	contentLength int64
	bar *progressbar.ProgressBar
}


func (d *Downloader) Download(){

	// check header
	req, err := http.NewRequest(http.MethodHead, d.url, nil)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	d.contentLength = resp.ContentLength
	acceptRanges := resp.Header.Get("Accept-Ranges")

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




	if acceptRanges != "bytes" {
		d.concurrencyN = 1
		log.Printf("%v, partial request is not supported, reset concurrencyN=1", d.url)
	}

	// set concurrencyN and partSize
	var wg sync.WaitGroup
	wg.Add(d.concurrencyN)
	partSize  := int(d.contentLength) / d.concurrencyN
	// log.Printf("downloader: %v", d)

	// download
	for i:=0; i < d.concurrencyN; i++ {
		rangeStart := i*partSize
		rangeEnd := rangeStart + partSize - 1
		if i == d.concurrencyN -1 {
			rangeEnd = int(d.contentLength)
		}
		filename := fmt.Sprintf("%v-%v", d.filename, i)

		go func() {
			defer wg.Done()

			downloadReq, err := http.NewRequest(http.MethodGet, d.url, nil)
			if err != nil {
				log.Fatal(err)
			}
			downloadReq.Header.Set("Range", fmt.Sprintf("bytes=%v-%v", rangeStart, rangeEnd))

			d.download(downloadReq, filename)

		}()
	}
	wg.Wait()
	d.merge()


}


func (d *Downloader)download(req *http.Request, filename string){
	// log.Printf("start downloading, range: %v, filename: %v\n", req.Header.Get("Range"), filename)
	resp, err := d.client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()


	file, err := os.OpenFile(filename, os.O_CREATE | os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	buf := make([]byte, 128*1024)
	if _, err := io.CopyBuffer(io.MultiWriter(file, d.bar), resp.Body, buf); err != nil && err != io.EOF {
		log.Fatal(err)
	}
	// log.Println("finish downloading", filename)

}


func (d *Downloader) merge() {
	dstFile, err := os.OpenFile(d.filename, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer dstFile.Close()

	for i := 0; i < d.concurrencyN; i++ {
		srcFilename := fmt.Sprintf("%v-%v", d.filename, i)
		srcFile, err := os.Open(srcFilename)
		if err != nil {
			log.Fatal(err)
		}

		buf := make([]byte, 128*1024)
		if _, err := io.CopyBuffer(dstFile, srcFile, buf); err != nil && err != io.EOF {
			log.Fatal(err)
		}
		srcFile.Close()

		os.Remove(srcFilename)

	}
}

