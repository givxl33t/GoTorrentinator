package bittorrent

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/givxl33t/bittorrent-client-go/peer"
)

// pieceJob includes metadata on a single piece to be downloaded
type pieceJob struct {
	Index  int
	Length int
	Hash   [20]byte
}

// pieceResult contains the downloaded piece bytes and its index
type pieceResult struct {
	Index     int
	FilePiece []byte
}

// Run the peer to peer download process concurrently getting the pieces
// outDir defaults to the current directory, "./"
func (d *Download) Run(outDir string) error {
	if outDir == "" {
		outDir = "./"
	}

	// make job queue that matches the size of the number of pieces
	jobQueue := make(chan pieceJob, len(d.Torrent.PieceHashes))
	results := make(chan pieceResult)

	// start "worker" goroutine for each peer client to grap jobs off of the queue
	for _, p := range d.PeerClients {
		p := p
		go func() {
			defer p.Close()
			for job := range jobQueue {
				pieceBuf, err := p.GetPiece(job.Index, job.Length, job.Hash)
				if err != nil {
					// place job back on queue
					jobQueue <- job
					// iff the client didn't have the piece, just continue
					if errors.Is(err, peer.ErrNotInBitfield) {
						continue
					}
					// otherwise stop listening to jobQueue, defer will cleanup cliennt
					fmt.Printf("disconnecting from %s after error: %s\n", p.Addr().String(), err.Error())
					return
				}
				results <- pieceResult{
					Index:     job.Index,
					FilePiece: pieceBuf,
				}
			}
		}()
	}

	// send all jobs to jobQueue channel
	for i, hash := range d.Torrent.PieceHashes {
		// all pieces are the full size except for the last piece
		length := d.Torrent.PieceLength
		if i == len(d.Torrent.PieceHashes)-1 {
			length = d.Torrent.Length - d.Torrent.PieceLength*(len(d.Torrent.PieceHashes)-1)
		}
		jobQueue <- pieceJob{
			Index:  i,
			Length: length,
			Hash:   hash,
		}
	}

	// merge results into buffer
	buf := make([]byte, d.Torrent.Length)
	for i := 0; i < len(d.Torrent.PieceHashes); i++ {
		piece := <-results

		// copy into final buffer
		copy(buf[piece.Index*d.Torrent.PieceLength:], piece.FilePiece)
		fmt.Printf("%0.2f%% complete\n", float64(i)/float64(len(d.Torrent.PieceHashes))*100)
	}

	// close job queue
	close(jobQueue)

	// write buffer to separate files
	var usedBytes int
	for _, file := range d.Torrent.Files {
		outPath := filepath.Join(outDir, file.Path)

		fmt.Printf("writing %d bytes to %s\n", file.Length, outPath)
		// ensure directory exists
		dir := filepath.Dir(outPath)
		_, err := os.Stat(dir)
		if os.IsNotExist(err) {
			err := os.MkdirAll(dir, os.ModePerm)
			if err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		// check integrity if hashes were provided
		fileRaw := buf[usedBytes : usedBytes+file.Length]
		if file.SHA1Hash != "" {
			hash := sha1.Sum(fileRaw)
			if !bytes.Equal(hash[:], []byte(file.SHA1Hash)) {
				return fmt.Errorf("%q failed SHA-1 hash mismatch", file.Path)
			}
		}
		if file.MD5Hash != "" {
			hash := md5.Sum(fileRaw)
			if !bytes.Equal(hash[:], []byte(file.MD5Hash)) {
				return fmt.Errorf("%q failed MD5 hash mismatch", file.Path)
			}
		}

		// write to file
		err = os.WriteFile(outPath, fileRaw, os.ModePerm)
		usedBytes += file.Length
		if err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
	}

	return nil
}
