package torrentparser

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/zeebo/bencode"
)

// AppendMetadata adds the metadata (aka the dictionary of a torrent file)
// It must be called after torrentfile.New() is invoked with a magnet link
// source with the metadata acquire from a peer in the swarm
func (t *TorrentFile) AppendMetadata(metadata []byte) error {
	var info bencodeInfo
	err := bencode.DecodeBytes(metadata, &info)
	if err != nil {
		return fmt.Errorf("unmarshalling info dict: %w", err)
	}

	// SHA-1 hash the entire info dictionary to get the info_hash
	t.InfoHash = sha1.Sum(metadata)

	// split the Pieces blob into the 20-byte SHA-1 hashes for comparison later
	const hashLen = 20 // length of a SHA-1 hash
	if len(info.Pieces)%hashLen != 0 {
		return errors.New("invalid length for info pieces")
	}
	t.PieceHashes = make([][20]byte, len(info.Pieces)/hashLen)
	for i := 0; i < len(t.PieceHashes); i++ {
		piece := info.Pieces[i*hashLen : (i+1)*hashLen]
		copy(t.PieceHashes[i][:], piece)
	}

	t.PieceLength = info.PieceLength

	// either Length OR Files field must be present (but not both)
	if info.Length == 0 && len(info.Files) == 0 {
		return fmt.Errorf("invalid torrent file info dict: no length OR files")
	}

	if info.Length != 0 {
		t.Files = append(t.Files, File{
			Length: info.Length,
			Path:   info.Name,
		})
		t.Length = info.Length
	} else {
		for _, f := range info.Files {
			subPaths := append([]string{info.Name}, f.Path...)
			t.Files = append(t.Files, File{
				Length:   f.Length,
				Path:     filepath.Join(subPaths...),
				SHA1Hash: f.SHA1Hash,
				MD5Hash:  f.MD5Hash,
			})
			t.Length += f.Length
		}
	}

	return nil
}
