package torrentparser

import (
	"bytes"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// parses a torrent magnet link
func ParseMagnetLink(magnetLink string) (TorrentFile, error) {
	u, err := url.Parse(magnetLink)
	if err != nil {
		return TorrentFile{}, fmt.Errorf("error parsing magnet link: %v", err)
	}

	xts := u.Query()["xt"]
	if len(xts) == 0 {
		return TorrentFile{}, fmt.Errorf("missing `xt` field on magnet link")
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
				raw, err := base32.StdEncoding.DecodeString(encodedInfoHash)
				if err != nil {
					return TorrentFile{}, fmt.Errorf("base32 decoding xt field: %w", err)
				}
				copy(infoHash[:], raw[:])
			default:
				return TorrentFile{}, fmt.Errorf("unimplemented xt field length %d", len(encodedInfoHash))
			}
		}
	}

	if bytes.Equal(infoHash[:], make([]byte, 20)) {
		return TorrentFile{}, fmt.Errorf("no xt field found, btmh unimplemented")
	}

	trs := u.Query()["tr"]
	if len(trs) == 0 {
		return TorrentFile{}, fmt.Errorf("no tracker urls in magnet link, DHT/PEX unimplemented")
	}

	return TorrentFile{
		InfoHash:    infoHash,
		TrackerURLs: trs,
		Name:        u.Query().Get("dn"),
	}, nil
}
