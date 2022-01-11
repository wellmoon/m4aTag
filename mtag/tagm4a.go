package mtag

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

type TagInfo struct {
	Artist  string
	Name    string
	Album   string
	PicPath string
	Year    string
	Comment string
}

type TagBufInfo struct {
	TagName string
	Buf     *bytes.Buffer
}

// overwrite : true - modify old file, false - rename old file with .old suffix
func UpdateM4aTag(overwrite bool, filePath string, title string, artist string, album string, comment string, picPath string) error {
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
	file, err := os.Open(filePath)
	if err != nil {
		panic("open m4a file error")
	}
	defer file.Close()

	b, err := readBytes(file, 11)
	if err != nil {
		return err
	}
	_, err = file.Seek(-11, io.SeekCurrent)
	if err != nil {
		return err
	}
	if string(b[4:8]) != "ftyp" {
		return errors.New("not support this file type")
	}
	tagInfo := &TagInfo{}
	tagInfo.Artist = artist
	tagInfo.Name = title
	tagInfo.PicPath = picPath
	tagInfo.Album = album
	tagInfo.Comment = comment
	list, err := SplitTopTag(file)
	if err != nil {
		return err
	}

	if overwrite {
		err = os.Remove(filePath)
	} else {
		err = os.Rename(filePath, filePath+".old")
	}

	if err != nil {
		return err
	}
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	mdat := ""
	for _, tagb := range list {
		tag := tagb.TagName
		buf := tagb.Buf
		if tag == "mdat" {
			mdat = "mdat"
		}
		if tag == "moov" {
			needUpdateStco := false
			if len(mdat) == 0 {
				// mdat tag after moov tag, need update stco
				needUpdateStco = true
			}
			buf, err = createMoov(buf, filePath, *tagInfo, needUpdateStco)
			if err != nil {
				return err
			}
		}
		_, err = f.Write(buf.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

func ReadM4aTag(filePath string) (*TagInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		panic("open m4a file error")
	}
	defer file.Close()

	b, err := readBytes(file, 11)
	if err != nil {
		return nil, err
	}
	_, err = file.Seek(-11, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	if string(b[4:8]) != "ftyp" {
		return nil, errors.New("not support this file type")
	}
	list, err := SplitTopTag(file)
	if err != nil {
		return nil, err
	}

	var moov *TagBufInfo
	for _, tag := range list {
		if tag.TagName != "moov" {
			continue
		}
		moov = tag
	}

	if moov == nil {
		return nil, errors.New("parse error")
	}
	return getMetaFromMoov(moov.Buf)
}

func SplitTopTag(r io.ReadSeeker) ([]*TagBufInfo, error) {
	var cBuf []byte = make([]byte, 4)
	var tagSize int
	var err error
	var tagBufInfo *TagBufInfo
	var tagBuf *bytes.Buffer
	list := make([]*TagBufInfo, 0)
	for {
		tagSize, err = readInt(r)
		if err != nil {
			break
		}
		_, err = io.ReadFull(r, cBuf)
		if err != nil {
			return nil, err
		}
		tagBufInfo = &TagBufInfo{}
		tagBufInfo.TagName = string(cBuf)
		tagBuf, err = createBufByTag(r, cBuf, tagSize)
		if err != nil {
			return nil, err
		}
		tagBufInfo.Buf = tagBuf
		list = append(list, tagBufInfo)
	}
	return list, nil
}

func createBufByTag(r io.ReadSeeker, tagName []byte, tagSize int) (*bytes.Buffer, error) {
	if tagSize > 100*1000000 {
		return nil, errors.New("makeslice: len out of range")
	}
	b := bytes.NewBuffer(int2Bytes(tagSize))
	b.Write(tagName)

	var t []byte
	if 128 >= tagSize-8 {
		t = make([]byte, tagSize-8)
		_, err := io.ReadFull(r, t)
		if err != nil {
			return nil, err
		}
		b.Write(t)
	} else {
		var from int = 1
		var delta int = 128
		for {
			if from*delta < tagSize-8 {
				t = make([]byte, delta)
				_, err := io.ReadFull(r, t)
				if err != nil {
					return nil, err
				}
				b.Write(t)
				from++
			} else {
				size := tagSize - 8 - (from-1)*delta
				t = make([]byte, size)
				_, err := io.ReadFull(r, t)
				if err != nil {
					return nil, err
				}
				b.Write(t)
				break
			}
		}
	}

	return b, nil
}

func createMoov(moov *bytes.Buffer, filePath string, tagInfo TagInfo, needUpdateStco bool) (*bytes.Buffer, error) {
	var cBuf []byte = make([]byte, 4)
	var tagSize int
	var err error
	var tag string

	var mvhdBuf *bytes.Buffer
	var trakBuf *bytes.Buffer
	var metaSize int
	r := bytes.NewReader(moov.Bytes())
	list := make([]*TagBufInfo, 0)
	for {
		tagSize, err = readInt(r)
		if err != nil {
			break
		}

		_, err = io.ReadFull(r, cBuf)
		if err != nil {
			return nil, err
		}
		tag = string(cBuf)
		if tag == "mvhd" {
			mvhdBuf, err = createBufByTag(r, cBuf, tagSize)
			if err != nil {
				return nil, err
			}
			list = append(list, &TagBufInfo{tag, mvhdBuf})
			continue
		} else if tag == "trak" {
			trakBuf, err = createBufByTag(r, cBuf, tagSize)
			if err != nil {
				return nil, err
			}
			list = append(list, &TagBufInfo{tag, trakBuf})
			continue
		} else if tag == "udta" || tag == "moov" {
			continue
		} else {
			// skip this tag
			if tag == "meta" {
				metaSize = tagSize
				r.Seek(int64(tagSize-8), io.SeekCurrent)
			} else {
				tempBuf, err := createBufByTag(r, cBuf, tagSize)
				if err != nil {
					return nil, err
				}
				list = append(list, &TagBufInfo{tag, tempBuf})
			}
		}
	}
	udtaBuf, newMetaSize := createUdta(tagInfo)
	if needUpdateStco {
		trakBuf = modifyStco(trakBuf, newMetaSize-metaSize)
		for _, b := range list {
			if b.TagName == "trak" {
				b.Buf = trakBuf
			}
		}
	}

	// moovLength := mvhdBuf.Len() + trakBuf.Len() + udtaBuf.Len() + 8
	moovLength := 8
	for _, b := range list {
		moovLength = moovLength + b.Buf.Len()
	}
	moovLength = moovLength + udtaBuf.Len()
	moovBuf := bytes.NewBuffer(int2Bytes(moovLength))
	moovBuf.WriteString("moov")
	for _, b := range list {
		moovBuf.Write(b.Buf.Bytes())
	}
	// moovBuf.Write(mvhdBuf.Bytes())
	// moovBuf.Write(trakBuf.Bytes())
	moovBuf.Write(udtaBuf.Bytes())
	return moovBuf, nil
}

func getMetaFromMoov(moov *bytes.Buffer) (*TagInfo, error) {
	var cBuf []byte = make([]byte, 4)
	var tagSize int
	var err error
	var tag string

	var result *TagInfo = &TagInfo{}
	r := bytes.NewReader(moov.Bytes())
	for {
		tagSize, err = readInt(r)
		if err != nil {
			break
		}
		if tagSize == 0 {
			break
		}
		_, err = io.ReadFull(r, cBuf)
		if err != nil {
			return nil, err
		}
		tag = string(cBuf)
		if tag == "moov" ||
			tag == "udta" ||
			tag == "ilst" {
			continue
		} else if tag == "meta" {
			r.Seek(int64(4), io.SeekCurrent)
			continue
		} else if cBuf[0] == byte(0xA9) {
			tmp := make([]byte, tagSize-8)
			io.ReadFull(r, tmp)
			if cBuf[1] == 'a' && cBuf[2] == 'l' && cBuf[3] == 'b' {
				result.Album = getValue(tmp)
			} else if cBuf[1] == 'A' && cBuf[2] == 'R' && cBuf[3] == 'T' {
				result.Artist = getValue(tmp)
			} else if cBuf[1] == 'n' && cBuf[2] == 'a' && cBuf[3] == 'm' {
				result.Name = getValue(tmp)
			} else if cBuf[1] == 'c' && cBuf[2] == 'm' && cBuf[3] == 't' {
				result.Comment = getValue(tmp)
			}
			continue
		} else {
			r.Seek(int64(tagSize-8), io.SeekCurrent)
		}
	}
	return result, nil
}

func getValue(buf []byte) string {
	var cBuf []byte = make([]byte, 4)
	var tagSize int
	var err error
	var tag string
	r := bytes.NewReader(buf)
	for {
		tagSize, err = readInt(r)
		if err != nil {
			return ""
		}
		_, err = io.ReadFull(r, cBuf)
		if err != nil {
			return ""
		}
		tag = string(cBuf)
		if tag == "data" {
			tmp := make([]byte, tagSize-8-8)
			r.Seek(int64(8), io.SeekCurrent)
			_, err = io.ReadFull(r, tmp)
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string((tmp)))
		} else {
			r.Seek(int64(tagSize-8), io.SeekCurrent)
		}
	}
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

	// tooBuf := createCopyTag("too", []byte("Lavf57.83.100"))
	hdlrBuf := createHdlr()

	ilstLenth := ___length + artistBuf.Len() + nameBuf.Len() + albumBuf.Len() + commentBuf.Len() + 8
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
	// ilstBuf.Write(tooBuf.Bytes())

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
