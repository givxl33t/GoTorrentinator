package torrentfile

import (
	"fmt"
	"net/url"
)

// parses a torrent magnet link
func ParseMagnetLink(magnetLink string) (*TorrentFile, error) {
	link, err := url.Parse(magnetLink)
	if err != nil {
		return nil, fmt.Errorf("failed to parse magnet link: %w", err)
	}

	// extract query parameter xt
	xts := link.Query()["xt"]
	if len(xts) != 1 {
		return nil, fmt.Errorf("invalid magnet link: %s", magnetLink)
	}

	// var InfoHash [20]byte
	for _, xt := range xts {
		// TODO parse xt to extract the info hash
		fmt.Println(xt)
	}

	return nil, nil
}
