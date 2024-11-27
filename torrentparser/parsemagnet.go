package torrentparser

import (
	"encoding/base32"
	"fmt"
	"net/url"
	"strings"
)

// parses a torrent magnet link
func ParseMagnetLink(magnetLink string) (TorrentFile, error) {
	link, err := url.Parse(magnetLink)
	if err != nil {
		return TorrentFile{}, fmt.Errorf("failed to parse magnet link: %w", err)
	}

	// extract query parameter xt
	xts := link.Query()["xt"]
	if len(xts) != 1 {
		return TorrentFile{}, fmt.Errorf("invalid magnet link: %s", magnetLink)
	}

	var infoHash [20]byte
	for _, xt := range xts {
		// parse xt to extract the info hash
		if strings.HasPrefix(xt, "urn:btih:") {
			encodedInfoHash := strings.TrimPrefix(xt, "urn:btih:")
			if len(xt) != 40 {
				return TorrentFile{}, fmt.Errorf("invalid magnet link: %s", magnetLink)
			}
			// validates info hash format as base32
			raw, err := base32.HexEncoding.DecodeString(encodedInfoHash)
			if err != nil {
				return TorrentFile{}, fmt.Errorf("invalid magnet link: %s", magnetLink)
			}
			// copy slice from raw to infoHash
			copy(infoHash[:], raw[:])
		}
	}

	trackerURLs := link.Query()["tr"]
	if len(trackerURLs) != 1 {
		return TorrentFile{}, fmt.Errorf("invalid magnet link: %s", magnetLink)
	}

	return TorrentFile{
		TrackerURLs: trackerURLs,
		InfoHash:    infoHash,
		Name:        link.Query().Get("dn"),
	}, nil
}
