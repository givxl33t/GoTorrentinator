package main

import (
	"flag"

	"github.com/givxl33t/bittorrent-client-go/bittorrent"
)

func main() {
	source := flag.String("source", "", "path to torrent file or magnet link")
	outDir := flag.String("out", "./", "path to output directory")
	flag.Parse()

	if *source == "" {
		panic("source flag is required")
	}

	d, err := bittorrent.NewDownload(*source)
	if err != nil {
		panic("starting download: " + err.Error())
	}

	err = d.Run(*outDir)
	if err != nil {
		panic("running download: " + err.Error())
	}
}
