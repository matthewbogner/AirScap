package vens

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/image/tiff"
)

func cornerImage(gray bool) image.Image {
	b := image.Rect(0, 0, 67, 49) // neither square nor MCU-aligned
	colors := []color.Color{
		color.RGBA{240, 20, 20, 255}, color.RGBA{20, 220, 20, 255},
		color.RGBA{20, 20, 240, 255}, color.RGBA{240, 220, 20, 255},
	}
	if gray {
		colors = []color.Color{color.Gray{0}, color.Gray{80}, color.Gray{160}, color.Gray{255}}
	}
	dst := image.NewRGBA(b)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			i := 0
			if x >= b.Dx()/2 {
				i++
			}
			if y >= b.Dy()/2 {
				i += 2
			}
			dst.Set(x, y, colors[i])
		}
	}
	if gray {
		g := image.NewGray(b)
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				g.Set(x, y, dst.At(x, y))
			}
		}
		return g
	}
	return dst
}

func encodedCorners(t *testing.T, format string) []byte {
	t.Helper()
	var buf bytes.Buffer
	var err error
	if format == "tiff" {
		err = tiff.Encode(&buf, cornerImage(true), &tiff.Options{Compression: tiff.Deflate})
	} else {
		err = jpeg.Encode(&buf, cornerImage(format == "gray"), &jpeg.Options{Quality: 100})
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func checkRotation(t *testing.T, original, rotated []byte, lossless bool) {
	t.Helper()
	src, format, err := image.Decode(bytes.NewReader(original))
	if err != nil {
		t.Fatal(err)
	}
	dst, gotFormat, err := image.Decode(bytes.NewReader(rotated))
	if err != nil {
		t.Fatal(err)
	}
	if dst.Bounds() != src.Bounds() || gotFormat != format {
		t.Fatalf("bounds/format changed: %v %s -> %v %s", src.Bounds(), format, dst.Bounds(), gotFormat)
	}
	b := src.Bounds()
	points := []image.Point{{4, 4}, {b.Max.X - 5, 4}, {4, b.Max.Y - 5}, {b.Max.X - 5, b.Max.Y - 5}}
	if lossless {
		points = nil
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				points = append(points, image.Pt(x, y))
			}
		}
	}
	for _, p := range points {
		want := color.RGBAModel.Convert(src.At(b.Max.X-1-p.X, b.Max.Y-1-p.Y)).(color.RGBA)
		got := color.RGBAModel.Convert(dst.At(p.X, p.Y)).(color.RGBA)
		for i, pair := range [][2]uint8{{got.R, want.R}, {got.G, want.G}, {got.B, want.B}} {
			delta := int(pair[0]) - int(pair[1])
			tolerance := 12
			if lossless {
				tolerance = 0
			}
			if delta < -tolerance || delta > tolerance {
				t.Errorf("pixel %v channel %d: got %v want %v", p, i, got, want)
			}
		}
	}
}

func TestRotatePage180(t *testing.T) {
	for _, format := range []string{"jpeg", "gray", "tiff"} {
		t.Run(format, func(t *testing.T) {
			data := encodedCorners(t, format)
			rotated, err := rotatePage180(data)
			if err != nil {
				t.Fatal(err)
			}
			checkRotation(t, data, rotated, format == "tiff")
		})
	}
}

func TestRotatePage180Invalid(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("not JPEG"), {0xff, 0xd8, 0xff, 0xe0, 0, 20}} {
		if out, err := rotatePage180(data); err == nil || out != nil {
			t.Fatalf("invalid image returned data=%x err=%v", out, err)
		}
	}
}

func TestRotateJPEGMetadata(t *testing.T) {
	data := encodedCorners(t, "jpeg")
	// JFIF: 300 x 200 DPI; also retain a comment.
	metadata := []byte{0xff, 0xe0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 2, 1, 1, 44, 0, 200, 0, 0,
		0xff, 0xfe, 0, 6, 't', 'e', 's', 't'}
	data = append(append(append([]byte{}, data[:2]...), metadata...), data[2:]...)
	rotated, err := rotatePage180(data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jpegMetadata(rotated)
	if err != nil || !bytes.Equal(got, metadata) {
		t.Fatalf("metadata changed: %x, err=%v", got, err)
	}
}

func TestRotateTIFFResolution(t *testing.T) {
	for _, bigEndian := range []bool{false, true} {
		data := encodedCorners(t, "tiff")
		// Rewrite just the IFD and resolution rationals in big-endian order
		// when requested. The Deflate pixel stream is byte-order independent.
		if bigEndian {
			off := binary.LittleEndian.Uint32(data[4:8])
			n := int(binary.LittleEndian.Uint16(data[off : off+2]))
			for i := 0; i < n; i++ {
				e := data[int(off)+2+i*12 : int(off)+2+(i+1)*12]
				tag, typ := binary.LittleEndian.Uint16(e), binary.LittleEndian.Uint16(e[2:])
				count, val := binary.LittleEndian.Uint32(e[4:]), binary.LittleEndian.Uint32(e[8:])
				if typ == 5 {
					for j := 0; j < 8; j += 4 {
						v := binary.LittleEndian.Uint32(data[int(val)+j:])
						binary.BigEndian.PutUint32(data[int(val)+j:], v)
					}
				}
				if typ == 3 && count == 1 {
					val = uint32(binary.LittleEndian.Uint16(e[8:])) << 16
				}
				binary.BigEndian.PutUint16(e, tag)
				binary.BigEndian.PutUint16(e[2:], typ)
				binary.BigEndian.PutUint32(e[4:], count)
				binary.BigEndian.PutUint32(e[8:], val)
			}
			copy(data[:4], []byte{'M', 'M', 0, 42})
			binary.BigEndian.PutUint32(data[4:], off)
			binary.BigEndian.PutUint16(data[off:], uint16(n))
		}
		order, fields, err := tiffResolutionFields(data)
		if err != nil {
			t.Fatal(err)
		}
		order.PutUint32(fields[282], 300)
		order.PutUint32(fields[283], 200)
		rotated, err := rotatePage180(data)
		if err != nil {
			t.Fatal(err)
		}
		outOrder, outFields, err := tiffResolutionFields(rotated)
		if err != nil {
			t.Fatal(err)
		}
		for _, tag := range []uint16{282, 283} {
			if outOrder.Uint32(outFields[tag]) != order.Uint32(fields[tag]) || outOrder.Uint32(outFields[tag][4:]) != order.Uint32(fields[tag][4:]) {
				t.Errorf("resolution tag %d changed", tag)
			}
		}
		if outOrder.Uint16(outFields[296]) != order.Uint16(fields[296]) {
			t.Error("resolution unit changed")
		}
		checkRotation(t, data, rotated, true)
	}
}

func TestTIFFResolutionValidation(t *testing.T) {
	data := encodedCorners(t, "tiff")
	order, fields, err := tiffResolutionFields(data)
	if err != nil {
		t.Fatal(err)
	}
	order.PutUint32(fields[282][4:], 0)
	if out, err := rotatePage180(data); err == nil || out != nil {
		t.Fatalf("zero resolution denominator: data=%x err=%v", out, err)
	}
	for _, data := range [][]byte{nil, {'I', 'I', 42, 0, 255, 255, 255, 255}, {'I', 'I', 42, 0, 8, 0, 0, 0, 255, 255}} {
		if _, _, err := tiffResolutionFields(data); err == nil {
			t.Errorf("invalid TIFF accepted: %x", data)
		}
	}
}

func TestRotationDoesNotChangeWireConfig(t *testing.T) {
	cfg := DefaultScanConfig()
	before := MarshalScanConfig([8]byte{}, cfg)
	cfg.Rotate180 = true
	if !bytes.Equal(before, MarshalScanConfig([8]byte{}, cfg)) {
		t.Fatal("host-side rotation changed scanner protocol configuration")
	}
}

// fakeScan serves the real StartScan/NextPage/RunScan protocol over localhost.
// Exact requests are checked, including duplex side flags and transfer indices.
func fakeScan(t *testing.T, cfg ScanConfig, images [][]byte) *DataChannel {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	done := make(chan error, 1)
	t.Cleanup(func() {
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("fake scanner did not finish")
		}
	})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		welcome := make([]byte, WelcomeSize)
		copy(welcome[4:8], Magic[:])
		if _, err := conn.Write(welcome); err != nil {
			done <- err
			return
		}
		response := func() []byte {
			r := make([]byte, 64)
			binary.BigEndian.PutUint32(r, uint32(len(r)))
			return r
		}
		exchange := func(request, reply []byte) error {
			got := make([]byte, len(request))
			if _, err := io.ReadFull(conn, got); err != nil {
				return err
			}
			if !bytes.Equal(got, request) {
				return fmt.Errorf("unexpected scanner request: %x want %x", got, request)
			}
			_, err := conn.Write(reply)
			return err
		}
		run := func() error {
			var token [8]byte
			for _, req := range [][]byte{MarshalGetScanSettings(token), MarshalScanConfig(token, cfg), MarshalPrepareScan(token), MarshalGetStatus(token), MarshalWaitForScan(token)} {
				if err := exchange(req, response()); err != nil {
					return err
				}
			}
			sides := 1
			if cfg.Duplex {
				sides = 2
			}
			for i, data := range images {
				if i > 0 && i%sides == 0 {
					if err := exchange(MarshalGetStatus(token), response()); err != nil {
						return err
					}
					if err := exchange(MarshalWaitForScan(token), response()); err != nil {
						return err
					}
				}
				r := make([]byte, PageHeaderSize+len(data))
				binary.BigEndian.PutUint32(r, uint32(len(r)))
				copy(r[4:8], Magic[:])
				binary.BigEndian.PutUint32(r[12:16], PageTypeFinal)
				copy(r[PageHeaderSize:], data)
				if err := exchange(MarshalPageTransfer(token, i, 0, cfg.Duplex && i%2 == 1), r); err != nil {
					return err
				}
				if err := exchange(MarshalGetPageMetadata(token), response()); err != nil {
					return err
				}
			}
			// Invalid image processing ends the session before the next feed.
			if len(images) > 0 && bytes.Equal(images[len(images)-1], []byte("invalid")) && cfg.Rotate180 {
				return exchange(MarshalEndScan(token), response())
			}
			if err := exchange(MarshalGetStatus(token), response()); err != nil {
				return err
			}
			r := response()
			binary.BigEndian.PutUint32(r[WaitRespStatusOffset:], 1)
			if err := exchange(MarshalWaitForScan(token), r); err != nil {
				return err
			}
			return exchange(MarshalEndScan(token), response())
		}
		done <- run()
	}()
	return NewDataChannel("127.0.0.1", uint16(ln.Addr().(*net.TCPAddr).Port), [8]byte{})
}

func TestScanRotationPaths(t *testing.T) {
	for _, batch := range []bool{false, true} {
		for _, rotate := range []bool{false, true} {
			for _, format := range []string{"jpeg", "gray", "tiff"} {
				t.Run(fmt.Sprintf("batch=%v/rotate=%v/%s", batch, rotate, format), func(t *testing.T) {
					data := encodedCorners(t, format)
					// Distinct front/back content catches unintended reordering.
					other, err := rotatePage180(data)
					if err != nil {
						t.Fatal(err)
					}
					images := [][]byte{data, other, data, other}
					cfg := DefaultScanConfig()
					cfg.Rotate180 = rotate
					dc := fakeScan(t, cfg, images)
					var pages, callbacks []Page
					if batch {
						pages, err = dc.RunScan(cfg, func(p Page) { callbacks = append(callbacks, p) })
					} else {
						var session *ScanSession
						session, err = dc.StartScan(cfg)
						if err != nil {
							t.Fatal(err)
						}
						defer session.Close()
						for {
							p, nextErr := session.NextPage()
							if nextErr == io.EOF {
								break
							}
							if nextErr != nil {
								t.Fatal(nextErr)
							}
							pages = append(pages, p)
						}
					}
					if err != nil || len(pages) != len(images) {
						t.Fatalf("pages=%d err=%v", len(pages), err)
					}
					for i, p := range pages {
						if p.Sheet != i/2 || p.Side != i%2 || p.PixelSize != nil {
							t.Errorf("page %d metadata changed: %+v", i, p)
						}
						if rotate {
							checkRotation(t, images[i], p.JPEG, format == "tiff")
						} else if !bytes.Equal(p.JPEG, images[i]) {
							t.Error("disabled rotation changed original bytes")
						}
						if batch && (len(callbacks) != len(pages) || !bytes.Equal(callbacks[i].JPEG, p.JPEG) || callbacks[i].Sheet != p.Sheet || callbacks[i].Side != p.Side) {
							t.Error("callback differs from returned page")
						}
					}
				})
			}
		}
	}
}

func TestScanRotationEmptyAndInvalid(t *testing.T) {
	for _, rotate := range []bool{false, true} {
		cfg := DefaultScanConfig()
		cfg.Duplex = false
		cfg.Rotate180 = rotate
		dc := fakeScan(t, cfg, [][]byte{nil, []byte("invalid")})
		var callbacks []Page
		pages, err := dc.RunScan(cfg, func(p Page) { callbacks = append(callbacks, p) })
		if rotate {
			if err == nil || !strings.Contains(err.Error(), "rotate sheet 1 side 0: decode image:") || len(pages) != 1 || len(callbacks) != 1 {
				t.Fatalf("pages=%d callbacks=%d err=%v", len(pages), len(callbacks), err)
			}
		} else if err != nil || len(pages) != 2 || !bytes.Equal(pages[1].JPEG, []byte("invalid")) {
			t.Fatalf("disabled processing altered passthrough: pages=%d err=%v", len(pages), err)
		}
		if len(pages[0].JPEG) != 0 {
			t.Error("blank-page sentinel changed")
		}
	}
}
