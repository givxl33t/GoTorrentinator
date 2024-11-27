package peer

import (
	"fmt"

	"github.com/zeebo/bencode"
)

// implementationn of extension protocol for BEP0010

type extendedHandshake struct {
	M struct {
		// value doubles as the extended message ID for metadata requests
		Metadata int `bencode:"m"`
	} `bencode:"m"`
	MetadataSize int `bencode:"metadata_size"`
}

func (p *Client) receiveExtendedHandshake() error {
	msg, err := p.receiveMessage()
	if err != nil {
		return fmt.Errorf("receiving extended handshake: %w", err)
	}

	// Allow 50 read retries if a non-extended message is received
	// to anticipate bitfield, unchoke, etc. Which doesn't need to
	// be handled as error
	for i := 0; i < 50 && messageID(msg.ID) != msgExtended; i++ {
		lastMsgID := messageID(msg.ID)
		msg, err = p.receiveMessage()
		if err != nil {
			return fmt.Errorf("retrying extended handshake after %s message: %w", lastMsgID, err)
		}
	}

	if messageID(msg.ID) != msgExtended {
		return fmt.Errorf("expected extended handshake, got %s", messageIDStrings[msg.ID])
	}

	const extendedHandshakeID uint8 = 0
	extMsgID := uint8(msg.Payload[0])
	payload := msg.Payload[1:]

	if extMsgID != extendedHandshakeID {
		return fmt.Errorf("expected extended handshake, got: %s", messageIDStrings[msg.ID])
	}

	var extendedResp extendedHandshake
	err = bencode.DecodeBytes(payload, &extendedResp)
	if err != nil {
		return fmt.Errorf("decoding extended handshake: %w", err)
	}

	p.ExtensionMetadata.messageID = extendedResp.M.Metadata
	p.ExtensionMetadata.metadataSize = extendedResp.MetadataSize

	return nil
}
