package torrentfile

import (
	"fmt"
	"os"

	"github.com/zeebo/bencode"
)

// parses a raw torrent file
func ParseTorrentFile(path string) (*TorrentFile, error) {
	// open torrent file
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// decode the top-level bencoded data
	var bencodeTorrentData bencodeTorrent
	if err := bencode.NewDecoder(file).Decode(&bencodeTorrentData); err != nil {
		return nil, fmt.Errorf("failed to decode bencoded data: %w", err)
	}

	// collect tracker URLs
	var trackerURLs []string
	trackerURLs = append(trackerURLs, bencodeTorrentData.Announce)
	for _, announceList := range bencodeTorrentData.AnnounceList {
		trackerURLs = append(trackerURLs, announceList...)
	}

	// populate torrentFile struct
	torrentFile := &TorrentFile{
		TrackerURLs: trackerURLs,
		Name:        path,
	}

	err = torrentFile.AppendMetadata(bencodeTorrentData.Info)
	if err != nil {
		return nil, fmt.Errorf("failed to append metadata: %w", err)
	}

	return torrentFile, nil
}
