package tlv

import (
	"bytes"
	"encoding/binary"
	"io"
)

// ByteSize is the size of a field in bytes. Used to define the size of the type and length field in a message.
type ByteSize int

const (
	OneByte    ByteSize = 1
	TwoBytes   ByteSize = 2
	FourBytes  ByteSize = 4
	EightBytes ByteSize = 8
)

type Record struct {
	Payload []byte
	Type    uint
}

// Codec is the configuration for a specific TLV encoding/decoding tasks.
type Codec struct {

	// TypeBytes defines the size in bytes of the message type field.
	TypeBytes ByteSize

	// LenBytes defines the size in bytes of the message length field.
	LenBytes ByteSize
}

// Writer encodes records into TLV format using a Codec and writes them into a provided io.Writer
type Writer struct {
	writer io.Writer
	codec  *Codec
}

func NewWriter(w io.Writer, codec *Codec) *Writer {
	return &Writer{
		codec:  codec,
		writer: w,
	}
}

// Write encodes records into TLV format using a Codec and writes them into a provided io.Writer
func (w *Writer) Write(rec *Record) error {
	err := writeUint(w.writer, w.codec.TypeBytes, rec.Type)
	if err != nil {
		return err
	}

	ulen := uint(len(rec.Payload))
	err = writeUint(w.writer, w.codec.LenBytes, ulen)
	if err != nil {
		return err
	}

	_, err = w.writer.Write(rec.Payload)
	return err
}

func writeUint(w io.Writer, b ByteSize, i uint) error {
	var num interface{}
	switch b {
	case OneByte:
		num = uint8(i)
	case TwoBytes:
		num = uint16(i)
	case FourBytes:
		num = uint32(i)
	case EightBytes:
		num = uint64(i)
	}
	return binary.Write(w, binary.BigEndian, num)
}

// Reader decodes records from TLV format using a Codec from provided io.Reader
type Reader struct {
	codec  *Codec
	reader io.Reader
}

func NewReader(reader io.Reader, codec *Codec) *Reader {
	return &Reader{codec: codec, reader: reader}
}

func (r *Reader) GetTypeAndLen() (uint, uint, error) {
	// get type
	typeBytes := make([]byte, r.codec.TypeBytes)
	_, err := io.ReadAtLeast(r.reader, typeBytes, int(r.codec.TypeBytes))
	if err != nil {
		return 0, 0, err
	}
	typ := readUint(typeBytes, r.codec.TypeBytes)

	// get len
	payloadLenBytes := make([]byte, r.codec.LenBytes)
	_, err = io.ReadAtLeast(r.reader, payloadLenBytes, int(r.codec.LenBytes))
	if err != nil && err != io.EOF {
		return 0, 0, err
	}
	payloadLen := readUint(payloadLenBytes, r.codec.LenBytes)

	if err == io.EOF && payloadLen != 0 {
		return 0, 0, err
	}

	return typ, payloadLen, nil

}

// Next tries to read a single Record from the io.Reader
func (r *Reader) Next() (*Record, error) {
	// get type
	typeBytes := make([]byte, r.codec.TypeBytes)
	_, err := r.reader.Read(typeBytes)
	if err != nil {
		return nil, err
	}
	typ := readUint(typeBytes, r.codec.TypeBytes)

	// get len
	payloadLenBytes := make([]byte, r.codec.LenBytes)
	_, err = r.reader.Read(payloadLenBytes)
	if err != nil && err != io.EOF {
		return nil, err
	}
	payloadLen := readUint(payloadLenBytes, r.codec.LenBytes)

	if err == io.EOF && payloadLen != 0 {
		return nil, err
	}

	// get value
	v := make([]byte, payloadLen)
	_, err = r.reader.Read(v)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return &Record{
		Type:    typ,
		Payload: v,
	}, nil

}

func readUint(b []byte, sz ByteSize) uint {
	reader := bytes.NewReader(b)
	switch sz {
	case OneByte:
		var i uint8
		binary.Read(reader, binary.BigEndian, &i)
		return uint(i)
	case TwoBytes:
		var i uint16
		binary.Read(reader, binary.BigEndian, &i)
		return uint(i)
	case FourBytes:
		var i uint32
		binary.Read(reader, binary.BigEndian, &i)
		return uint(i)
	case EightBytes:
		var i uint64
		binary.Read(reader, binary.BigEndian, &i)
		return uint(i)
	default:
		return 0
	}
}
