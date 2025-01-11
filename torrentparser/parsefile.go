package torrentparser

import (
	"fmt"
	"os"

	"github.com/zeebo/bencode"
)

// parses a raw torrent file
func ParseTorrentFile(path string) (TorrentFile, error) {
	path = os.ExpandEnv(path)
	f, err := os.Open(path)
	if err != nil {
		return TorrentFile{}, err
	}

	var btor bencodeTorrent
	err = bencode.NewDecoder(f).Decode(&btor)
	if err != nil {
		return TorrentFile{}, fmt.Errorf("unmarshalling file: %w", err)
	}

	var trackerURLs []string
	for _, list := range btor.AnnounceList {
		trackerURLs = append(trackerURLs, list...)
	}
	// BEP0012, only use `announce` if `announce-list` is not present
	if len(trackerURLs) == 0 {
		trackerURLs = append(trackerURLs, btor.Announce)
	}
	tf := TorrentFile{
		TrackerURLs: trackerURLs,
		Name:        path,
	}

	err = tf.AppendMetadata(btor.Info)
	if err != nil {
		return TorrentFile{}, fmt.Errorf("parsing metadata: %w", err)
	}

	return tf, nil
}
