package mysql_replication_listener

import (
	"bytes"
	"io"
)

type (
	packReader struct {
		conn io.Reader
	}

	packWriter struct {
		conn io.Writer
	}

	pack struct {
		sequence byte
		length   uint32
		buff     []byte
		*bytes.Buffer
	}
)

func newPackReader(conn io.Reader) *packReader {
	return &packReader{
		conn: conn,
	}
}

func newPackWriter(conn io.Writer) *packWriter {
	return &packWriter{
		conn: conn,
	}
}

func (w *packWriter) flush(p *pack) error {
	_, err := w.conn.Write(p.packBytes())
	return err
}

func newPack() *pack {
	pack := &pack{
		buff: make([]byte, 4),
	}

	pack.Buffer = bytes.NewBuffer(pack.buff)

	return pack
}

func (r *packReader) readNextPack() (*pack, error) {
	buff := make([]byte, 4)
	_, err := r.conn.Read(buff)
	if err != nil {
		return nil, err
	}
	var length uint32

	err = readThreeBytesUint32(buff[0:3], &length)
	if err != nil {
		return nil, err
	}

	pack := &pack{
		sequence: buff[3],
		length:   length,
		buff:     make([]byte, length),
	}

	pack.Buffer = bytes.NewBuffer(pack.buff)

	_, err = r.conn.Read(pack.buff)
	if err != nil {
		return nil, err
	}

	return pack, nil
}

func (r *pack) getSequence() byte {
	return r.sequence
}

func (r *pack) setSequence(s byte) {
	r.sequence = s
}

func (r *pack) readByte(dest *byte) (err error) {
	*dest, err = r.ReadByte()
	return
}

func (r *pack) readUint16(dest *uint16) error {
	readUint16(r.Buffer.Next(2), dest)
	return nil
}

func (r *pack) readThreeByteUint32(dest *uint32) error {
	readThreeBytesUint32(r.Buffer.Next(3), dest)
	return nil
}

func (r *pack) readUint32(dest *uint32) error {
	readUint32(r.Buffer.Next(4), dest)
	return nil
}

func (r *pack) readUint64(dest *uint64) error {
	readUint64(r.Buffer.Next(8), dest)
	return nil
}

func (r *pack) readNilString() ([]byte, error) {
	buff, err := r.ReadBytes(byte(0))

	if err != nil {
		return []byte{}, err
	}

	return buff[0 : len(buff)-1], nil
}

func (r *pack) readStringLength() ([]byte, error) {
	var (
		length uint64
		null   bool
	)

	err := r.readIntLengthOrNil(&length, &null)

	if err != nil {
		return []byte{}, err
	}

	if length == 0 {
		return []byte{}, nil
	}

	return r.Next(int(length)), nil
}

func (r *pack) readIntLengthOrNil(value *uint64, null *bool) error {
	lb, err := r.ReadByte()

	if err != nil {
		return err
	}

	switch lb {
	case 0xFB:
		*null = true
	case 0xFC:
		var val uint16
		r.readUint16(&val)
		*value = uint64(val)
	case 0xFD:
		var val uint32
		r.readThreeByteUint32(&val)
		*value = uint64(val)
	case 0xFE:
		r.readUint64(value)
	default:
		*value = uint64(lb)
	}
	return nil
}

func (r *pack) writeUInt16(data uint16) error {
	buff := make([]byte, 2)

	writeUInt16(buff, data)
	_, err := r.Write(buff)
	return err
}

func (r *pack) writeThreeByteUInt32(data uint32) error {
	buff := make([]byte, 3)

	writeThreeByteUInt32(buff, data)
	_, err := r.Write(buff)
	return err
}

func (r *pack) writeUInt32(data uint32) error {
	buff := make([]byte, 4)

	writeUInt32(buff, data)
	_, err := r.Write(buff)
	return err
}

func (r *pack) writeUInt64(data uint64) error {
	buff := make([]byte, 8, 8)

	writeUInt64(buff, data)

	_, err := r.Write(buff)
	return err
}

func (r *pack) writeStringNil(data string) error {
	_, err := r.Write([]byte(data))
	if err != nil {
		return err
	}

	err = r.WriteByte(byte(0))
	return err
}

func (r *pack) writeStringLength(data string) error {
	length := writeLengthInt(uint64(len(data)))

	_, err := r.Write(length)
	if err != nil {
		return err
	}

	_, err = r.Write([]byte(data))
	if err != nil {
		return err
	}

	return err
}

func (r *pack) packBytes() []byte {
	buff := r.Bytes()
	writeThreeByteUInt32(buff, uint32(len(buff)-4))
	buff[3] = r.getSequence()
	return buff
}

func (r *pack) isError() error {
	if r.buff[0] == _MYSQL_ERR {
		errPack := &errPacket{}
		readUint16(r.buff[1:3], &errPack.code)
		errPack.description = r.buff[3:]
		return errPack
	}
	return nil
}

func (r *pack) isEOF() bool {
	return r.buff[0] == _MYSQL_EOF
}