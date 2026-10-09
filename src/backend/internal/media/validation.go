package media

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// 多帧 GIF 的压缩体积不能代表解码内存；以下预算保护 2C2G 单机的上传进程。
const (
	maxGIFFrames      = 1000
	maxGIFFramePixels = 100_000_000
)

type imageInfo struct {
	mimeType   string
	extension  string
	size       int64
	width      int
	height     int
	checksum   [32]byte
	contentMD5 string
}

func inspectImage(file io.ReadSeeker) (imageInfo, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return imageInfo{}, ErrInvalidImage
	}
	sha256Hasher := sha256.New()
	md5Hasher := md5.New()
	size, err := io.Copy(io.MultiWriter(sha256Hasher, md5Hasher), io.LimitReader(file, MaxFileSize+1))
	if err != nil {
		return imageInfo{}, ErrInvalidImage
	}
	if size > MaxFileSize {
		return imageInfo{}, ErrTooLarge
	}
	if size == 0 {
		return imageInfo{}, ErrInvalidImage
	}

	mimeType, extension, format, err := detectImageFormat(file)
	if err != nil {
		return imageInfo{}, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return imageInfo{}, ErrInvalidImage
	}
	config, decodedFormat, err := image.DecodeConfig(file)
	if err != nil || decodedFormat != format {
		return imageInfo{}, ErrInvalidImage
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > MaxDimension || config.Height > MaxDimension || int64(config.Width)*int64(config.Height) > MaxPixels {
		return imageInfo{}, ErrInvalidDimensions
	}

	hasEXIF, err := containsEXIF(file, format)
	if err != nil {
		return imageInfo{}, ErrInvalidImage
	}
	if hasEXIF {
		return imageInfo{}, ErrEXIFForbidden
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return imageInfo{}, ErrInvalidImage
	}
	if format == "gif" {
		// GIF 必须校验全部帧；先读取帧描述限制解码预算，避免小文件膨胀耗尽单机内存。
		if err := checkGIFFrameBudget(file); err != nil {
			return imageInfo{}, err
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return imageInfo{}, ErrInvalidImage
		}
		if _, err := gif.DecodeAll(file); err != nil {
			return imageInfo{}, ErrInvalidImage
		}
	} else if _, fullFormat, err := image.Decode(file); err != nil || fullFormat != format {
		return imageInfo{}, ErrInvalidImage
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return imageInfo{}, ErrInvalidImage
	}

	var checksum [32]byte
	copy(checksum[:], sha256Hasher.Sum(nil))
	return imageInfo{
		mimeType: mimeType, extension: extension, size: size,
		width: config.Width, height: config.Height, checksum: checksum,
		contentMD5: base64.StdEncoding.EncodeToString(md5Hasher.Sum(nil)),
	}, nil
}

func detectImageFormat(file io.ReadSeeker) (string, string, string, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", "", ErrInvalidImage
	}
	header := make([]byte, 12)
	if _, err := io.ReadFull(file, header); err != nil {
		return "", "", "", ErrInvalidImage
	}
	switch {
	case bytes.HasPrefix(header, []byte{0xff, 0xd8, 0xff}):
		return "image/jpeg", "jpg", "jpeg", nil
	case bytes.Equal(header[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		return "image/png", "png", "png", nil
	case string(header[:4]) == "RIFF" && string(header[8:12]) == "WEBP":
		return "image/webp", "webp", "webp", nil
	case bytes.Equal(header[:6], []byte("GIF87a")) || bytes.Equal(header[:6], []byte("GIF89a")):
		return "image/gif", "gif", "gif", nil
	case bytes.Equal(header[:2], []byte("BM")):
		return "image/bmp", "bmp", "bmp", nil
	default:
		return "", "", "", ErrUnsupportedFormat
	}
}

func containsEXIF(file io.ReadSeeker, format string) (bool, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	switch format {
	case "jpeg":
		return jpegContainsEXIF(file)
	case "png":
		return chunkContainsEXIF(file, 8, binary.BigEndian, "eXIf", false)
	case "webp":
		return chunkContainsEXIF(file, 12, binary.LittleEndian, "EXIF", true)
	case "bmp", "gif":
		return false, nil
	default:
		return false, ErrUnsupportedFormat
	}
}

func checkGIFFrameBudget(reader io.Reader) error {
	header := make([]byte, 13)
	if _, err := io.ReadFull(reader, header); err != nil {
		return ErrInvalidImage
	}
	if header[10]&0x80 != 0 {
		if _, err := io.CopyN(io.Discard, reader, int64(3<<(int(header[10]&7)+1))); err != nil {
			return ErrInvalidImage
		}
	}
	frames := 0
	var totalPixels int64
	var block [1]byte
	var descriptor [9]byte
	for {
		if _, err := io.ReadFull(reader, block[:]); err != nil {
			return ErrInvalidImage
		}
		switch block[0] {
		case 0x2c: // 图像描述符列出每一帧的实际像素矩形。
			if _, err := io.ReadFull(reader, descriptor[:]); err != nil {
				return ErrInvalidImage
			}
			width := int64(binary.LittleEndian.Uint16(descriptor[4:6]))
			height := int64(binary.LittleEndian.Uint16(descriptor[6:8]))
			frames++
			totalPixels += width * height
			if width == 0 || height == 0 || frames > maxGIFFrames || totalPixels > maxGIFFramePixels {
				return ErrInvalidDimensions
			}
			if descriptor[8]&0x80 != 0 {
				if _, err := io.CopyN(io.Discard, reader, int64(3<<(int(descriptor[8]&7)+1))); err != nil {
					return ErrInvalidImage
				}
			}
			if _, err := io.CopyN(io.Discard, reader, 1); err != nil {
				return ErrInvalidImage
			}
			if err := skipGIFSubBlocks(reader); err != nil {
				return err
			}
		case 0x21: // 扩展块可能包含播放延迟或注释，长度由子块序列决定。
			if _, err := io.CopyN(io.Discard, reader, 1); err != nil {
				return ErrInvalidImage
			}
			if err := skipGIFSubBlocks(reader); err != nil {
				return err
			}
		case 0x3b:
			if frames == 0 {
				return ErrInvalidImage
			}
			return nil
		default:
			return ErrInvalidImage
		}
	}
}

func skipGIFSubBlocks(reader io.Reader) error {
	var length [1]byte
	for {
		if _, err := io.ReadFull(reader, length[:]); err != nil {
			return ErrInvalidImage
		}
		if length[0] == 0 {
			return nil
		}
		if _, err := io.CopyN(io.Discard, reader, int64(length[0])); err != nil {
			return ErrInvalidImage
		}
	}
}

func jpegContainsEXIF(reader io.Reader) (bool, error) {
	buffered := bufio.NewReader(reader)
	start := make([]byte, 2)
	if _, err := io.ReadFull(buffered, start); err != nil || !bytes.Equal(start, []byte{0xff, 0xd8}) {
		return false, ErrInvalidImage
	}
	for {
		prefix, err := buffered.ReadByte()
		if err != nil {
			return false, err
		}
		if prefix != 0xff {
			return false, ErrInvalidImage
		}
		marker, err := buffered.ReadByte()
		if err != nil {
			return false, err
		}
		for marker == 0xff {
			marker, err = buffered.ReadByte()
			if err != nil {
				return false, err
			}
		}
		if marker == 0xda || marker == 0xd9 {
			return false, nil
		}
		lengthBytes := make([]byte, 2)
		if _, err := io.ReadFull(buffered, lengthBytes); err != nil {
			return false, err
		}
		length := int(binary.BigEndian.Uint16(lengthBytes)) - 2
		if length < 0 {
			return false, ErrInvalidImage
		}
		if marker == 0xe1 {
			prefix := make([]byte, min(length, 6))
			if _, err := io.ReadFull(buffered, prefix); err != nil {
				return false, err
			}
			if bytes.Equal(prefix, []byte{'E', 'x', 'i', 'f', 0, 0}) {
				return true, nil
			}
			length -= len(prefix)
		}
		if _, err := io.CopyN(io.Discard, buffered, int64(length)); err != nil {
			return false, err
		}
	}
}

func chunkContainsEXIF(reader io.Reader, headerSize int64, order binary.ByteOrder, target string, padded bool) (bool, error) {
	if _, err := io.CopyN(io.Discard, reader, headerSize); err != nil {
		return false, err
	}
	for {
		header := make([]byte, 8)
		if _, err := io.ReadFull(reader, header); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return false, nil
			}
			return false, err
		}
		if string(header[:4]) == target && padded {
			return true, nil
		}
		if string(header[4:8]) == target && !padded {
			return true, nil
		}
		var size uint32
		if padded {
			size = order.Uint32(header[4:8])
		} else {
			size = order.Uint32(header[:4]) + 4 // PNG 数据后还包含 CRC。
		}
		if padded && size%2 == 1 {
			size++
		}
		if _, err := io.CopyN(io.Discard, reader, int64(size)); err != nil {
			return false, err
		}
	}
}
