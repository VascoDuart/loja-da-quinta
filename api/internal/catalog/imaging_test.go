package catalog

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// withExif insere um segmento APP1 (EXIF) logo a seguir ao SOI de um JPEG,
// com a orientação dada e texto que imita coordenadas GPS.
func withExif(jpg []byte, orientation uint16) []byte {
	var tiff bytes.Buffer
	tiff.WriteString("MM\x00\x2a")
	_ = binary.Write(&tiff, binary.BigEndian, uint32(8)) // IFD0 logo a seguir
	_ = binary.Write(&tiff, binary.BigEndian, uint16(1)) // uma entrada
	_ = binary.Write(&tiff, binary.BigEndian, uint16(0x0112))
	_ = binary.Write(&tiff, binary.BigEndian, uint16(3)) // SHORT
	_ = binary.Write(&tiff, binary.BigEndian, uint32(1))
	_ = binary.Write(&tiff, binary.BigEndian, orientation)
	_ = binary.Write(&tiff, binary.BigEndian, uint16(0))
	_ = binary.Write(&tiff, binary.BigEndian, uint32(0)) // sem mais IFDs
	tiff.WriteString("GPSLatitude 38.7223 N GPSLongitude 9.1393 W")

	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	seg = append(seg, payload...)

	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	return append(out, jpg[2:]...)
}

// pngClaiming devolve um PNG válido cujo cabeçalho diz ter w×h píxeis.
func pngClaiming(t *testing.T, w, h uint32) []byte {
	t.Helper()
	b := encodePNG(t, solid(1, 1, color.White))
	// IHDR: 8 bytes de assinatura, 4 de tamanho, "IHDR", largura, altura…
	binary.BigEndian.PutUint32(b[16:20], w)
	binary.BigEndian.PutUint32(b[20:24], h)
	binary.BigEndian.PutUint32(b[29:33], crc32.ChecksumIEEE(b[12:29]))
	return b
}

func TestProcessesJPEGIntoTwoSizes(t *testing.T) {
	out, err := ProcessImage(encodeJPEG(t, solid(1600, 1200, color.RGBA{200, 80, 20, 255})))
	if err != nil {
		t.Fatal(err)
	}
	if out.Large.Width != 800 || out.Large.Height != 600 {
		t.Errorf("large = %dx%d, queria 800x600", out.Large.Width, out.Large.Height)
	}
	if out.Thumb.Width != 160 || out.Thumb.Height != 120 {
		t.Errorf("thumb = %dx%d, queria 160x120", out.Thumb.Width, out.Thumb.Height)
	}
	for _, v := range []EncodedImage{out.Large, out.Thumb} {
		if sniffFormat(v.Data) != "jpeg" {
			t.Error("a saída tem de ser sempre JPEG")
		}
	}
	if out.Large.SHA256 == out.Thumb.SHA256 {
		t.Error("cada versão tem o seu hash")
	}
}

func TestSmallImagesAreNotUpscaled(t *testing.T) {
	out, err := ProcessImage(encodePNG(t, solid(120, 90, color.White)))
	if err != nil {
		t.Fatal(err)
	}
	if out.Large.Width != 120 || out.Thumb.Width != 120 {
		t.Errorf("uma imagem pequena não se aumenta: large %d, thumb %d", out.Large.Width, out.Thumb.Width)
	}
}

func TestPNGTransparencyBecomesWhite(t *testing.T) {
	out, err := ProcessImage(encodePNG(t, solid(10, 10, color.Transparent)))
	if err != nil {
		t.Fatal(err)
	}
	img, _ := jpeg.Decode(bytes.NewReader(out.Large.Data))
	r, g, b, _ := img.At(5, 5).RGBA()
	if r>>8 < 250 || g>>8 < 250 || b>>8 < 250 {
		t.Errorf("o fundo transparente devia ficar branco, ficou %d,%d,%d", r>>8, g>>8, b>>8)
	}
}

func TestDecodesWebP(t *testing.T) {
	// WebP sem perdas de 1×1 px.
	webp, _ := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	out, err := ProcessImage(webp)
	if err != nil {
		t.Fatal(err)
	}
	if sniffFormat(out.Large.Data) != "jpeg" || out.Large.Width != 1 {
		t.Errorf("o WebP devia virar um JPEG de 1 px: %+v", out.Large.Width)
	}
}

func TestRejectsWhatIsNotAnImage(t *testing.T) {
	var gifBuf bytes.Buffer
	_ = gif.Encode(&gifBuf, solid(4, 4, color.Black), nil)

	cases := map[string][]byte{
		"vazio":                       {},
		"texto":                       []byte("não sou uma imagem"),
		"SVG com script":              []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"HTML":                        []byte("<!doctype html><script>alert(1)</script>"),
		"GIF":                         gifBuf.Bytes(),
		"PDF":                         []byte("%PDF-1.7\n"),
		"extensão .jpg, conteúdo zip": []byte("PK\x03\x04qualquer-coisa"),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ProcessImage(data); !errors.Is(err, ErrUnsupportedImage) {
				t.Errorf("queria ErrUnsupportedImage, deu %v", err)
			}
		})
	}
}

func TestRejectsBrokenImages(t *testing.T) {
	jpg := encodeJPEG(t, solid(20, 20, color.White))
	cases := map[string][]byte{
		"só o cabeçalho JPEG": {0xFF, 0xD8, 0xFF, 0xE0, 0x00},
		"JPEG cortado a meio": jpg[:len(jpg)/2],
		"PNG com lixo":        append([]byte("\x89PNG\r\n\x1a\n"), []byte("lixo")...),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ProcessImage(data); !errors.Is(err, ErrInvalidImage) {
				t.Errorf("queria ErrInvalidImage, deu %v", err)
			}
		})
	}
}

func TestRejectsDecompressionBombBeforeDecoding(t *testing.T) {
	// 10000 × 5000 = 50 MP declarados num ficheiro de poucos bytes. Se fosse
	// descodificado, pediria 200 MB de memória.
	if _, err := ProcessImage(pngClaiming(t, 10000, 5000)); !errors.Is(err, ErrImageTooLarge) {
		t.Errorf("queria ErrImageTooLarge, deu %v", err)
	}
}

func TestRejectsFilesOver5MB(t *testing.T) {
	big := append(encodeJPEG(t, solid(10, 10, color.White)), make([]byte, MaxImageBytes)...)
	if _, err := ProcessImage(big); !errors.Is(err, ErrImageTooLarge) {
		t.Errorf("queria ErrImageTooLarge, deu %v", err)
	}
}

func TestPolyglotLosesHiddenContent(t *testing.T) {
	// Um JPEG válido com HTML colado no fim: alguns browsers antigos
	// adivinhavam o tipo e executavam-no.
	poly := append(encodeJPEG(t, solid(30, 30, color.White)), []byte("<html><script>alert(1)</script></html>")...)
	out, err := ProcessImage(poly)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Large.Data, []byte("<script>")) || bytes.Contains(out.Thumb.Data, []byte("<script>")) {
		t.Error("o conteúdo escondido tinha de desaparecer no reprocessamento")
	}
}

func TestExifAndGPSAreRemovedAndOrientationApplied(t *testing.T) {
	// Foto "deitada": 40×20 com orientação 6 (rodar 90° no sentido dos ponteiros).
	src := withExif(encodeJPEG(t, solid(40, 20, color.RGBA{10, 120, 30, 255})), 6)
	if exifOrientation("jpeg", src) != 6 {
		t.Fatal("o teste devia ter EXIF com orientação 6")
	}
	out, err := ProcessImage(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []EncodedImage{out.Large, out.Thumb} {
		if bytes.Contains(v.Data, []byte("Exif")) || bytes.Contains(v.Data, []byte("GPSLatitude")) {
			t.Error("o EXIF e as coordenadas GPS tinham de desaparecer")
		}
	}
	if out.Large.Width != 20 || out.Large.Height != 40 {
		t.Errorf("a foto devia ficar de pé (20x40), ficou %dx%d", out.Large.Width, out.Large.Height)
	}
}

func TestOrientationsMapCorners(t *testing.T) {
	// Píxel marcado no canto superior esquerdo de uma imagem 3×2.
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	mark := color.RGBA{255, 0, 0, 255}
	src.SetRGBA(0, 0, mark)
	want := map[int]image.Point{
		1: {0, 0}, 2: {2, 0}, 3: {2, 1}, 4: {0, 1},
		5: {0, 0}, 6: {1, 0}, 7: {1, 2}, 8: {0, 2},
	}
	for o, p := range want {
		if got := orient(src, o).RGBAAt(p.X, p.Y); got != mark {
			t.Errorf("orientação %d: o canto devia ir para %v", o, p)
		}
	}
}

func TestMalformedExifIsIgnored(t *testing.T) {
	jpg := encodeJPEG(t, solid(8, 8, color.White))
	broken := withExif(jpg, 6)
	broken[10] = 'X' // estraga a ordem dos bytes do TIFF
	if o := exifOrientation("jpeg", broken); o != 1 {
		t.Errorf("EXIF estragado devia valer orientação 1, deu %d", o)
	}
	if _, err := ProcessImage(broken); err != nil {
		t.Errorf("um EXIF estragado não impede a imagem: %v", err)
	}
}
