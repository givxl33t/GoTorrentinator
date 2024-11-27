package torrentparser

import (
	"encoding/base32"
	"encoding/hex"
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
		// todo implement v2 btmh (multihash)
		if strings.HasPrefix(xt, "urn:btih:") {
			encodedInfoHash := strings.TrimPrefix(xt, "urn:btih:")

			switch len(encodedInfoHash) {
			case 40:
				raw, err := hex.DecodeString(encodedInfoHash)
				if err != nil {
					return TorrentFile{}, fmt.Errorf("hex decoding xt field: %w", err)
				}
				copy(infoHash[:], raw[:])
			case 32:
				raw, err := base32.HexEncoding.DecodeString(encodedInfoHash)
				if err != nil {
					return TorrentFile{}, fmt.Errorf("base32 decoding xt field: %w", err)
				}
				copy(infoHash[:], raw[:])
			default:
				return TorrentFile{}, fmt.Errorf("unimplemented xt field length %d", len(encodedInfoHash))
			}
		}
	}

	trackerURLs := link.Query()["tr"]
	if len(trackerURLs) == 0 {
		return TorrentFile{}, fmt.Errorf("invalid magnet link: %s", magnetLink)
	}

	return TorrentFile{
		TrackerURLs: trackerURLs,
		InfoHash:    infoHash,
		Name:        link.Query().Get("dn"),
	}, nil
}
