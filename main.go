package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
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
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output `filepath/dirpath`",
			},
			&cli.StringFlag{
				Name:    "proxy",
				Aliases: []string{"p"},
				Usage:   "proxy url",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Value:   false,
				Usage:   "log to console",
			},
			&cli.IntFlag{
				Name:    "concurrency",
				Aliases: []string{"n"},
				Value:   runtime.NumCPU(),
				Usage:   "Concurrency `number`, default: the number of cpus",
			},
			&cli.IntFlag{
				Name:    "blockSize",
				Aliases: []string{"b"},
				Value:   100 * 1024 * 1024,
				Usage:   "block size, default: 100M",
			},
		},
		Action: func(c *cli.Context) error {
			url := c.Args().Get(0)
			if "" !=  c.String("url"){
				url = c.String("url")
			}

			downloader := &Downloader{
				concurrencyN: c.Int("concurrency"),
				url:          url,
				filepath:     c.String("output"),
				blockSize:    c.Int("blockSize"),
				client:       generateClient(c.String("proxy")),
			}

			log.SetFlags(log.LstdFlags | log.Lshortfile)
			if !c.Bool("verbose") {
				log.SetOutput(ioutil.Discard)
			}


			downloader.Download()
			return nil
		},
	}

	app.Run(os.Args)

}
