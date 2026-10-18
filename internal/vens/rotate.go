package vens

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"

	"golang.org/x/image/tiff"
)

// rotatePage180 retains the image format and dimensions. JPEG requires lossy
// recompression: Go's JPEG codec has no coefficient-domain transform API.
// TIFF pixels are encoded losslessly with Deflate (not the scanner's CCITT G4).
func rotatePage180(data []byte) ([]byte, error) {
	src, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if format != "jpeg" && format != "tiff" {
		return nil, fmt.Errorf("unsupported scan image format %q", format)
	}

	b := src.Bounds()
	var dst interface {
		image.Image
		Set(int, int, color.Color)
	}
	if _, gray := src.(*image.Gray); gray {
		dst = image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	} else {
		dst = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	}
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(x, y, src.At(b.Max.X-1-x, b.Max.Y-1-y))
		}
	}

	var out bytes.Buffer
	if format == "tiff" {
		if err := tiff.Encode(&out, dst, &tiff.Options{Compression: tiff.Deflate}); err != nil {
			return nil, fmt.Errorf("encode TIFF: %w", err)
		}
		// x/image/tiff writes a hard-coded 72 DPI. Restore the original X/Y
		// resolution and unit so clients and PDF generation retain page size.
		if err := copyTIFFResolution(out.Bytes(), data); err != nil {
			return nil, fmt.Errorf("preserve TIFF resolution: %w", err)
		}
		return out.Bytes(), nil
	}
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("encode JPEG: %w", err)
	}
	// jpeg.Encode emits no APP segments. Preserve scanner metadata (notably
	// JFIF density used by PDF generation), without copying old coding tables.
	metadata, err := jpegMetadata(data)
	if err != nil {
		return nil, err
	}
	result := make([]byte, 0, out.Len()+len(metadata))
	result = append(result, out.Bytes()[:2]...)
	result = append(result, metadata...)
	result = append(result, out.Bytes()[2:]...)
	return result, nil
}

func jpegMetadata(data []byte) ([]byte, error) {
	var metadata []byte
	for i := 2; i < len(data); {
		start := i
		if data[i] != 0xff {
			return nil, fmt.Errorf("invalid JPEG marker at offset %d", i)
		}
		for i < len(data) && data[i] == 0xff {
			i++
		}
		if i >= len(data) {
			return nil, fmt.Errorf("truncated JPEG marker")
		}
		marker := data[i]
		i++
		if marker == 0xda || marker == 0xd9 { // SOS or EOI
			return metadata, nil
		}
		if marker == 0x01 || marker >= 0xd0 && marker <= 0xd7 {
			continue // standalone markers
		}
		if i+2 > len(data) {
			return nil, fmt.Errorf("truncated JPEG segment length")
		}
		n := int(binary.BigEndian.Uint16(data[i : i+2]))
		if n < 2 || n > len(data)-i {
			return nil, fmt.Errorf("invalid JPEG segment length %d", n)
		}
		i += n
		if marker >= 0xe0 && marker <= 0xef || marker == 0xfe {
			metadata = append(metadata, data[start:i]...)
		}
	}
	return nil, fmt.Errorf("JPEG missing scan marker")
}

// tiffResolutionFields returns byte slices for the first IFD's resolution
// fields. Values are kept as rationals, without rounding or changing units.
func tiffResolutionFields(data []byte) (binary.ByteOrder, map[uint16][]byte, error) {
	if len(data) < 8 {
		return nil, nil, fmt.Errorf("truncated TIFF header")
	}
	var order binary.ByteOrder = binary.LittleEndian
	if string(data[:2]) == "MM" {
		order = binary.BigEndian
	} else if string(data[:2]) != "II" {
		return nil, nil, fmt.Errorf("invalid TIFF byte order")
	}
	off := uint64(order.Uint32(data[4:8]))
	if off+2 > uint64(len(data)) {
		return nil, nil, fmt.Errorf("invalid TIFF IFD offset")
	}
	n := uint64(order.Uint16(data[off : off+2]))
	if off+2+n*12+4 > uint64(len(data)) {
		return nil, nil, fmt.Errorf("truncated TIFF IFD")
	}
	fields := make(map[uint16][]byte)
	for i := uint64(0); i < n; i++ {
		entry := data[off+2+i*12 : off+2+(i+1)*12]
		tag := order.Uint16(entry[:2])
		if tag != 282 && tag != 283 && tag != 296 {
			continue
		}
		typ := order.Uint16(entry[2:4])
		if order.Uint32(entry[4:8]) != 1 {
			return nil, nil, fmt.Errorf("invalid TIFF resolution count")
		}
		if tag == 296 && typ == 3 { // ResolutionUnit, SHORT
			fields[tag] = entry[8:10]
		} else if tag != 296 && typ == 5 { // X/YResolution, RATIONAL
			value := uint64(order.Uint32(entry[8:12]))
			if value+8 > uint64(len(data)) || order.Uint32(data[value+4:value+8]) == 0 {
				return nil, nil, fmt.Errorf("invalid TIFF resolution rational")
			}
			fields[tag] = data[value : value+8]
		} else {
			return nil, nil, fmt.Errorf("invalid TIFF resolution type")
		}
	}
	return order, fields, nil
}

func copyTIFFResolution(dst, src []byte) error {
	srcOrder, srcFields, err := tiffResolutionFields(src)
	if err != nil {
		return err
	}
	dstOrder, dstFields, err := tiffResolutionFields(dst)
	if err != nil {
		return err
	}
	for tag, dest := range dstFields {
		value := srcFields[tag]
		if tag == 296 {
			unit := uint16(2) // default to inches
			if value != nil {
				unit = srcOrder.Uint16(value)
			}
			dstOrder.PutUint16(dest, unit)
		} else {
			num, den := uint32(0), uint32(1) // unknown, not the encoder's 72 DPI
			if value != nil {
				num, den = srcOrder.Uint32(value[:4]), srcOrder.Uint32(value[4:])
			}
			dstOrder.PutUint32(dest[:4], num)
			dstOrder.PutUint32(dest[4:], den)
		}
	}
	return nil
}
