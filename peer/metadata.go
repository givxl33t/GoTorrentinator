package peer

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/zeebo/bencode"
)

type metadataMessageType uint8

const (
	request metadataMessageType = iota
	data
	reject
)

type metadataMessage struct {
	Type      metadataMessageType `bencode:"msg_type"`
	Piece     int                 `bencode:"piece"`
	TotalSize int                 `bencode:"total_size,omitempty"`
}

// GetMetadata reqquests and receives the raw metadata/info dictionary from peer
func (p *Client) GetMetadata(infoHash [20]byte) ([]byte, error) {
	if p.ExtensionMetadata.messageID == 0 || p.ExtensionMetadata.metadataSize == 0 {
		return nil, fmt.Errorf("metadata not supported by peer")
	}
	defer p.Conn.SetDeadline(time.Time{})

	metadataBuf := make([]byte, p.ExtensionMetadata.metadataSize)

	const metadataPieceSize = 16384 // 16KiB

	var requested, received int
	for received < p.ExtensionMetadata.metadataSize {
		// request one piece at a time
		if requested <= received/metadataPieceSize {
			var buf bytes.Buffer
			// write the message id for the extension protocol
			buf.WriteByte(byte(p.ExtensionMetadata.messageID))

			msgRaw, err := bencode.EncodeBytes(metadataMessage{
				Type:  request,
				Piece: requested,
			})
			if err != nil {
				return nil, fmt.Errorf("encoding metadata request: %w", err)
			}
			buf.Write(msgRaw)

			p.Conn.SetDeadline(time.Now().Add(3 * time.Second))
			err = p.sendMessage(msgExtended, buf.Bytes())
			if err != nil {
				return nil, fmt.Errorf("sending metadata request: %w", err)
			}
			requested++
		}

		// update deadline for reading each piece
		p.Conn.SetDeadline(time.Now().Add(5 * time.Second))
		msg, err := p.receiveMessage()
		if err != nil {
			return nil, fmt.Errorf("receiving metadata: %w", err)
		}
		// ignore non-extended messages like unchoke,have
		if msg.ID != msgExtended {
			continue
		}

		extPayload := msg.Payload[1:]
		// read bytes until end of bencoded dictionary ("ee" ends total_size integer then dictionary)
		var dictRaw []byte
		for i := 1; i < len(extPayload); i++ {
			if string(extPayload[i-1:i+1]) == "ee" {
				dictRaw = extPayload[:i]
				break
			}
		}

		if len(dictRaw) == 0 {
			return nil, fmt.Errorf("got empty metadata dictionary")
		}

		var msgResp metadataMessage
		err = bencode.DecodeBytes(dictRaw, &msgResp)
		if err != nil {
			return nil, fmt.Errorf("decoding metadata: %w", err)
		}

		if msgResp.Type == reject {
			return nil, fmt.Errorf("got metadata reject message for piece %d", msgResp.Piece)
		}
		if msgResp.Type != data {
			return nil, fmt.Errorf("got unexpected metadata message type %d", msgResp.Type)
		}
		if msgResp.TotalSize != p.ExtensionMetadata.metadataSize {
			return nil, fmt.Errorf("got unexpected metadata size %d, want %d", msgResp.TotalSize, p.ExtensionMetadata.metadataSize)
		}

		// piece bytes are after the dictionary
		pieceRaw := extPayload[len(dictRaw):]

		// copy into metadata buffer & update the number of received bytes
		received += copy(metadataBuf[msgResp.Piece*metadataPieceSize:], pieceRaw[:])
	}

	// validate metadata via SHA-1
	hash := sha1.Sum(metadataBuf)
	if !bytes.Equal(hash[:], infoHash[:]) {
		return nil, fmt.Errorf("invalid metadata")
	}

	return metadataBuf, nil
}
