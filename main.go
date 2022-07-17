package main

import (
	"log"
	"net/http"
	"os"
	"path"
	"runtime"

	"github.com/urfave/cli/v2"
)

func main() {

	app := &cli.App{
		Name:  "downloader",
		Usage: "http concurrency downloader",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "url",
				Aliases:  []string{"u"},
				Usage:    "`URL` to download",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "output dirname/filename",
				Aliases: []string{"o"},
				Usage:   "Output `filename/dirname`",
			},
			&cli.IntFlag{
				Name:    "concurrency, default: the number of cpus",
				Aliases: []string{"n"},
				Value:   runtime.NumCPU(),
				Usage:   "Concurrency `number`",
			},
		},
		Action: func(c *cli.Context) error {

			downloader := &Downloader{
				concurrencyN: c.Int("concurrency"),
				url:          c.String("url"),
				filename:     c.String("output"),
				client:       &http.Client{},
			}

			if downloader.filename == "" {
				downloader.filename = path.Base(downloader.url)
			} else if file, err := os.Stat(downloader.filename); err == nil && file.IsDir() {
				downloader.filename = path.Join(downloader.filename, path.Base(downloader.url))
			}

			log.Printf("url: %v\nfilename: %v\nconcurrencyN: %v", downloader.url, downloader.filename, downloader.concurrencyN)
			downloader.Download()
			return nil
		},
	}

	app.Run(os.Args)

}
