package mtag

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
)

type TagInfo struct {
	Artist  string
	Name    string
	Album   string
	PicPath string
	Year    string
	Comment string
}

// createNewFile : true-rewrite old file, false-rename old file with .old suffix
func UpdateM4aTag(createNewFile bool, filePath string, title string, artist string, album string, comment string, picPath string) {
	//ftyp
	//moov
	//- mvhd
	//- trak
	//-  - tkhd
	//-  - edts
	//-  - mdia
	//- udta
	//-  - meta
	//			hdlr
	//			ilst
	//				----
	//				)ART
	//					data
	//				)nam
	//					data
	//				)alb
	//					data
	//				)too
	//free
	//mdat
	f, err := os.Open(filePath)
	if err != nil {
		panic("open m4a file error")
	}
	defer f.Close()

	b, err := readBytes(f, 11)
	if err != nil {
		return
	}
	_, err = f.Seek(-11, io.SeekCurrent)
	if err != nil {
		return
	}
	if string(b[4:8]) != "ftyp" {
		return
	}
	tagInfo := &TagInfo{}
	tagInfo.Artist = artist
	tagInfo.Name = title
	tagInfo.PicPath = picPath
	tagInfo.Album = album
	tagInfo.Comment = comment
	readAtoms(f, filePath, *tagInfo, createNewFile)
}

func createBufByTag(r io.ReadSeeker, tagName []byte, tagSize int) (*bytes.Buffer, error) {
	b := bytes.NewBuffer(int2Bytes(tagSize))
	b.Write(tagName)
	t := make([]byte, tagSize-8)
	_, err := io.ReadFull(r, t)
	if err != nil {
		return nil, err
	}
	b.Write(t)
	return b, nil
}

func readAtoms(r io.ReadSeeker, filePath string, tagInfo TagInfo, createNewFile bool) error {
	var cBuf []byte = make([]byte, 4)
	var tagSize int
	var err error
	var tag string

	var ftypBuf *bytes.Buffer
	var mvhdBuf *bytes.Buffer
	var trakBuf *bytes.Buffer
	var mdatBuf *bytes.Buffer
	var metaSize int
	for {
		tagSize, err = readInt(r)
		if err != nil {
			return err
		}
		_, err = io.ReadFull(r, cBuf)
		if err != nil {
			return err
		}
		tag = string(cBuf)
		if tag == "ftyp" {
			ftypBuf, err = createBufByTag(r, cBuf, tagSize)
			if err != nil {
				return err
			}
			continue
		} else if tag == "moov" {
			// create new moov tag
			continue
		} else if tag == "mvhd" {
			mvhdBuf, err = createBufByTag(r, cBuf, tagSize)
			if err != nil {
				return err
			}
			continue
		} else if tag == "trak" {
			trakBuf, err = createBufByTag(r, cBuf, tagSize)
			if err != nil {
				return err
			}
			continue
		} else if tag == "mdat" {
			mdatBuf, err = createBufByTag(r, cBuf, tagSize)
			if err != nil {
				return err
			}
			break
		} else if tag == "udta" {
			continue
		} else {
			// skip this tag
			if tag == "meta" {
				metaSize = tagSize
			}
			r.Seek(int64(tagSize-8), io.SeekCurrent)
		}
	}

	udtaBuf, newMetaSize := createUdta(tagInfo)
	trakBuf = modifyStco(trakBuf, newMetaSize-metaSize)

	moovLength := mvhdBuf.Len() + trakBuf.Len() + udtaBuf.Len() + 8
	moovBuf := bytes.NewBuffer(int2Bytes(moovLength))
	moovBuf.WriteString("moov")
	moovBuf.Write(mvhdBuf.Bytes())
	moovBuf.Write(trakBuf.Bytes())
	moovBuf.Write(udtaBuf.Bytes())

	if createNewFile {
		err = os.Remove(filePath)
	} else {
		os.Rename(filePath, filePath+".old")
	}

	if err != nil {
		return err
	}
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	freeBuf := createFree(8)
	_, err = f.Write(ftypBuf.Bytes())
	if err != nil {
		return err
	}
	_, err = f.Write(moovBuf.Bytes())
	if err != nil {
		return err
	}
	_, err = f.Write(freeBuf.Bytes())
	if err != nil {
		return err
	}
	_, err = f.Write(mdatBuf.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func modifyStco(trakBuf *bytes.Buffer, diff int) *bytes.Buffer {
	if diff == 0 {
		return trakBuf
	}
	// modify stco
	tempBytes := trakBuf.Bytes()
	idx := bytes.Index(tempBytes, []byte("stco"))

	idx = idx + 8
	truckNum := getInt(tempBytes[idx : idx+4])
	idx = idx + 4
	for i := 0; i < truckNum; i++ {
		oldLen := getInt(tempBytes[idx : idx+4])
		newLen := oldLen + diff
		b := int2Bytes(newLen)
		for j := 0; j < len(b); j++ {
			tempBytes[idx+j] = b[j]
		}
		idx = idx + 4
	}

	newBuf := bytes.NewBuffer(tempBytes)
	return newBuf
}

func createUdta(tagInfo TagInfo) (*bytes.Buffer, int) {
	meta := createMeta(tagInfo)
	udtaLength := meta.Len() + 8
	udtaBuf := bytes.NewBuffer(int2Bytes(udtaLength))
	udtaBuf.WriteString("udta")
	udtaBuf.Write(meta.Bytes())
	return udtaBuf, meta.Len()
}

func createMeta(tagInfo TagInfo) *bytes.Buffer {
	// create artist tag
	artistBuf := createCopyTag("ART", []byte(tagInfo.Artist))
	nameBuf := createCopyTag("nam", []byte(tagInfo.Name))
	albumBuf := createCopyTag("alb", []byte(tagInfo.Album))
	commentBuf := createCopyTag("cmt", []byte(tagInfo.Comment))
	picBytes, err := ReadFile(tagInfo.PicPath)
	var covrBuf *bytes.Buffer
	if err == nil {
		covrBuf = createCovr(picBytes)
	}

	meanBuf := createTag("mean", []byte("com.apple.iTunes"))
	subNameBuf := createTag("name", []byte("iTunMOVI"))
	subDataBuf := createSubData()

	___length := meanBuf.Len() + subNameBuf.Len() + subDataBuf.Len() + 8
	___buf := bytes.NewBuffer(int2Bytes(___length))
	___buf.WriteString("----")
	___buf.Write(meanBuf.Bytes())
	___buf.Write(subNameBuf.Bytes())
	___buf.Write(subDataBuf.Bytes())

	tooBuf := createCopyTag("too", []byte("Lavf57.83.100"))
	hdlrBuf := createHdlr()

	ilstLenth := ___length + artistBuf.Len() + nameBuf.Len() + albumBuf.Len() + commentBuf.Len() + tooBuf.Len() + 8
	if covrBuf != nil {
		ilstLenth = ilstLenth + covrBuf.Len()
	}
	ilstBuf := bytes.NewBuffer(int2Bytes(ilstLenth))
	ilstBuf.WriteString("ilst")
	ilstBuf.Write(___buf.Bytes())
	if covrBuf != nil {
		ilstBuf.Write(covrBuf.Bytes())
	}
	ilstBuf.Write(artistBuf.Bytes())
	ilstBuf.Write(albumBuf.Bytes())
	ilstBuf.Write(nameBuf.Bytes())
	ilstBuf.Write(commentBuf.Bytes())
	ilstBuf.Write(tooBuf.Bytes())

	metaLength := hdlrBuf.Len() + ilstLenth + 12
	metaBuf := bytes.NewBuffer(int2Bytes(metaLength))
	metaBuf.WriteString("meta")
	metaBuf.Write([]byte{0x00, 0x00, 0x00, 0x00})
	metaBuf.Write(hdlrBuf.Bytes())
	metaBuf.Write(ilstBuf.Bytes())
	// metaBuf.Write(freeBuf.Bytes())
	return metaBuf
}

func createCopyTag(tagName string, value []byte) *bytes.Buffer {
	length := len(value) + 16 + 8
	buf := bytes.NewBuffer(int2Bytes(length))
	buf.WriteByte(0xA9)
	buf.WriteString(tagName)
	buf.Write(int2Bytes(length - 8))
	buf.WriteString("data")
	buf.Write([]byte{0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
	buf.Write(value)
	return buf
}

func createCovr(value []byte) *bytes.Buffer {
	length := len(value) + 16 + 8
	buf := bytes.NewBuffer(int2Bytes(length))
	buf.WriteString("covr")
	buf.Write(int2Bytes(length - 8))
	buf.WriteString("data")
	buf.Write([]byte{0x00, 0x00, 0x00, 0xD0, 0x00, 0x00, 0x00, 0x00})
	buf.Write(value)
	return buf
}

func createTag(tagName string, value []byte) *bytes.Buffer {
	length := len(value) + 4 + 8
	buf := bytes.NewBuffer(int2Bytes(length))
	buf.WriteString(tagName)
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00})
	buf.Write(value)
	return buf
}

func createFree(len int) *bytes.Buffer {
	buf := bytes.NewBuffer(int2Bytes(len))
	buf.WriteString("free")
	for i := 0; i < len-8; i++ {
		buf.WriteByte(0x01)
	}
	return buf
}
func createSubData() *bytes.Buffer {
	length := 200
	buf := bytes.NewBuffer(int2Bytes(length))
	buf.WriteString("data")
	buf.Write([]byte{0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
	buf.WriteByte(0x0A)
	buf.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">")
	buf.WriteByte(0x0A)
	buf.WriteString("<plist version=\"1.0\"><dict></dict></plist>")
	return buf
}

func createHdlr() *bytes.Buffer {
	length := 33
	buf := bytes.NewBuffer(int2Bytes(length))
	buf.WriteString("hdlr")
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	buf.WriteString("mdirappl")
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	return buf
}

func getInt(b []byte) int {
	var n int
	for _, x := range b {
		n = n << 8
		n |= int(x)
	}
	return n
}

func int2Bytes(value int) []byte {
	b := make([]byte, 4)
	for i := 0; i < 4; i++ {
		b[4-i-1] = byte((value >> (8 * i)) & 0xFF)
	}
	return b
}

func readInt(r io.ReadSeeker) (int, error) {
	b := make([]byte, 4)
	_, err := io.ReadFull(r, b)
	if err != nil {
		return 0, err
	}
	return getInt(b), nil
}

func readBytes(r io.Reader, n uint) ([]byte, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func ReadFile(picFile string) ([]byte, error) {
	if len(picFile) == 0 {
		return nil, errors.New("file is null")
	}
	contents, err := ioutil.ReadFile(picFile)
	if err != nil {
		return nil, err
	}
	return contents, nil
}
