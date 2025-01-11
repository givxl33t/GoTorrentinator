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

// GetMetadata requests and receives the raw metadata/info dictionary from peer
func (p *Client) GetMetadata(infoHash [20]byte) ([]byte, error) {
	if p.ExtensionMetadata.messageID == 0 || p.ExtensionMetadata.metadataSize == 0 {
		return nil, fmt.Errorf("client does not support metadata extension")
	}
	defer p.Conn.SetDeadline(time.Time{})

	metadataBuf := make([]byte, p.ExtensionMetadata.metadataSize)

	const metadataPieceSize = 16384 // 16KiB

	var requested, received int
	for received < p.ExtensionMetadata.metadataSize {
		// request one piece at a time, in my experience, clients don't like backlogging/pipelining
		// metadata piece requests  and they'll end up sending the first piece only
		if requested <= received/metadataPieceSize {
			var buf bytes.Buffer
			// write the message id for the extension protocol
			buf.WriteByte(byte(p.ExtensionMetadata.messageID))

			msgRaw, err := bencode.EncodeBytes(metadataMessage{
				Type:  request,
				Piece: requested,
			})
			if err != nil {
				return nil, fmt.Errorf("bencoding metadata req: %w", err)
			}
			buf.Write(msgRaw)

			p.Conn.SetDeadline(time.Now().Add(time.Second * 3))
			err = p.sendMessage(msgExtended, buf.Bytes())
			if err != nil {
				return nil, fmt.Errorf("sending metadata request: %w", err)
			}
			requested++
		}

		// update deadline for reading each piece
		p.Conn.SetDeadline(time.Now().Add(time.Second * 5))
		msg, err := p.receiveMessage()
		if err != nil {
			return nil, fmt.Errorf("receiving metadata piece: %w", err)
		}
		// clients will send non-extended messages, like unchoke/have, ignore them
		if msg.ID != msgExtended {
			continue
		}

		// process the extended message http://www.bittorrent.org/beps/bep_0010.html
		extMsgID := uint8(msg.Payload[0])
		// expect extension message id to match, although in practice it's always been zero
		_ = extMsgID
		// if int(extMsgID) != p.ExtensionMetadata.messageID {
		// 	return fmt.Errorf("metadata extension ids do not match: want %d, got %d", p.ExtensionMetadata.messageID, extMsgID)
		// }

		extPayload := msg.Payload[1:]
		// read bytes until end of bencoded dictionary ("ee" ends total_size integer then dictionary)
		var dictRaw []byte
		for i := 1; i < len(extPayload); i++ {
			if string(extPayload[i-1:i+1]) == "ee" {
				dictRaw = extPayload[:i+1]
				break
			}
		}
		if len(dictRaw) == 0 {
			return nil, fmt.Errorf("malformed extension dictionary")
		}

		var msgResp metadataMessage
		err = bencode.DecodeBytes(dictRaw, &msgResp)
		if err != nil {
			return nil, fmt.Errorf("decoding bencoded extension dictionary: %w", err)
		}

		if msgResp.Type == reject {
			return nil, fmt.Errorf("got metadata reject message for piece %d", msgResp.Piece)
		}
		if msgResp.Type != data {
			return nil, fmt.Errorf("want data type (1), got %d", msgResp.Type)
		}
		if msgResp.TotalSize != p.ExtensionMetadata.metadataSize {
			return nil, fmt.Errorf("got metadata data.total_size %d, want %d", msgResp.TotalSize, p.ExtensionMetadata.metadataSize)
		}

		// piece bytes are after the dictionary
		pieceRaw := extPayload[len(dictRaw):]

		// copy into metadata buffer & update the number of received bytes
		received += copy(metadataBuf[msgResp.Piece*metadataPieceSize:], pieceRaw[:])
	}

	// validate metadata via SHA-1
	hash := sha1.Sum(metadataBuf)
	if !bytes.Equal(hash[:], infoHash[:]) {
		return nil, fmt.Errorf("metadata failed integrity check")
	}
	return metadataBuf, nil
}
