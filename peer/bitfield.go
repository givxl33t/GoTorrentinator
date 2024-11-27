package peer

// bitfield communicates which pieces a peer has and can send us
type bitfield []byte

func (b bitfield) HasPiece(index int) bool {
	if len(b) == 0 {
		return false
	}

	byteIndex := index / 8
	offset := index % 8

	mask := 1 << (7 - offset)

	return (byte(mask) & b[byteIndex]) != 0
}

func (b bitfield) SetPiece(index int) {
	byteIndex := index / 8
	// discard if index is out of range of bitfield
	if byteIndex >= len(b) {
		return
	}
	offset := index % 8
	mask := 1 << (7 - offset)
	b[byteIndex] |= byte(mask)
}
