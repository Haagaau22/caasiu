package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"

	"github.com/urfave/cli/v2"
)

func generateClient(proxy string) *http.Client {
	if proxy == "" {
		return &http.Client{}
	}

	proxyUrl, err := url.Parse(proxy)
	if err != nil {
		log.Fatal(err)
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		},
	}

}

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
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output `filename/dirname`",
			},
			&cli.StringFlag{
				Name:    "proxy",
				Aliases: []string{"p"},
				Usage:   "proxy url`",
			},
			&cli.IntFlag{
				Name:    "concurrency",
				Aliases: []string{"n"},
				Value:   runtime.NumCPU(),
				Usage:   "Concurrency `number`, default: the number of cpus",
			},
		},
		Action: func(c *cli.Context) error {

			downloader := &Downloader{
				concurrencyN: c.Int("concurrency"),
				url:          c.String("url"),
				filename:     c.String("output"),
				client:       generateClient(c.String("proxy")),
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
