package catalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // descodificador PNG
	"math"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // descodificador WebP (Go puro; não há codificador sem cgo)
)

// Limites e formato de saída das imagens (ADR-0014).
const (
	MaxImageBytes  = 5 << 20
	MaxImagePixels = 40_000_000
	LargeSide      = 800
	ThumbSide      = 160
	jpegQuality    = 82
)

// EncodedImage é uma versão já reprocessada em JPEG.
type EncodedImage struct {
	Data   []byte
	Width  int
	Height int
	SHA256 [32]byte
}

// ProcessedImage são as duas versões guardadas de cada imagem.
type ProcessedImage struct {
	Large EncodedImage
	Thumb EncodedImage
}

// ProcessImage valida e reprocessa um ficheiro enviado pelo gestor:
//
//  1. O formato vem dos primeiros bytes (JPEG, PNG ou WebP), nunca da
//     extensão nem do Content-Type. SVG, GIF, HTML e o resto dão
//     ErrUnsupportedImage.
//  2. As dimensões leem-se do cabeçalho antes de descodificar; mais de 40
//     megapíxeis dá ErrImageTooLarge (bomba de descompressão).
//  3. A imagem é descodificada, rodada segundo o EXIF e codificada de novo em
//     JPEG, com 800 e 160 px no lado maior. O original nunca se guarda: os
//     metadados (EXIF, GPS) e qualquer conteúdo escondido desaparecem.
//
// A transparência de PNG e WebP fica sobre fundo branco.
func ProcessImage(data []byte) (ProcessedImage, error) {
	if len(data) > MaxImageBytes {
		return ProcessedImage{}, ErrImageTooLarge
	}
	format := sniffFormat(data)
	if format == "" {
		return ProcessedImage{}, ErrUnsupportedImage
	}

	cfg, decoded, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || decoded != format {
		return ProcessedImage{}, ErrInvalidImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return ProcessedImage{}, ErrInvalidImage
	}
	if int64(cfg.Width)*int64(cfg.Height) > MaxImagePixels {
		return ProcessedImage{}, ErrImageTooLarge
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ProcessedImage{}, ErrInvalidImage
	}

	// Reduzir primeiro e só depois rodar: rodar 800 px custa muito menos do
	// que rodar a foto original de 40 MP.
	large := orient(fit(src, LargeSide), exifOrientation(format, data))
	thumb := fit(large, ThumbSide)

	l, err := encode(large)
	if err != nil {
		return ProcessedImage{}, err
	}
	t, err := encode(thumb)
	if err != nil {
		return ProcessedImage{}, err
	}
	return ProcessedImage{Large: l, Thumb: t}, nil
}

// sniffFormat reconhece o formato pelos bytes mágicos. Devolve o nome que o
// pacote image usa ("jpeg", "png", "webp"), ou "".
func sniffFormat(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "jpeg"
	case len(b) >= 8 && bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")):
		return "webp"
	}
	return ""
}

// fit reduz a imagem para caber num quadrado de side px (nunca aumenta) e
// desenha-a sobre branco, o que também tira a transparência.
func fit(src image.Image, side int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if longest := max(w, h); longest > side {
		scale := float64(side) / float64(longest)
		w = max(1, int(math.Round(float64(w)*scale)))
		h = max(1, int(math.Round(float64(h)*scale)))
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func encode(img *image.RGBA) (EncodedImage, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return EncodedImage{}, fmt.Errorf("codificar em JPEG: %w", err)
	}
	b := img.Bounds()
	return EncodedImage{Data: buf.Bytes(), Width: b.Dx(), Height: b.Dy(), SHA256: sha256.Sum256(buf.Bytes())}, nil
}

// orient aplica a orientação EXIF (1 a 8). As fotos de telemóvel vêm muitas
// vezes "deitadas", com a rotação só indicada no EXIF, que se vai descartar.
func orient(src *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 { // as orientações 5 a 8 trocam largura e altura
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range h {
		for x := range w {
			var dx, dy int
			switch o {
			case 2: // espelho horizontal
				dx, dy = w-1-x, y
			case 3: // 180°
				dx, dy = w-1-x, h-1-y
			case 4: // espelho vertical
				dx, dy = x, h-1-y
			case 5: // transposta
				dx, dy = y, x
			case 6: // 90° no sentido dos ponteiros
				dx, dy = h-1-y, x
			case 7: // transversa
				dx, dy = h-1-y, w-1-x
			case 8: // 90° no sentido contrário
				dx, dy = y, w-1-x
			}
			dst.SetRGBA(dx, dy, src.RGBAAt(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// exifOrientation lê a tag Orientation (0x0112) do EXIF de um JPEG. Devolve 1
// (normal) se não houver EXIF ou se estiver mal formado: o pior caso é uma
// foto deitada, nunca um erro.
func exifOrientation(format string, b []byte) int {
	if format != "jpeg" {
		return 1
	}
	i := 2 // depois do SOI
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			return 1
		}
		marker := b[i+1]
		if marker == 0xDA || marker == 0xD9 { // início dos dados da imagem ou fim
			return 1
		}
		size := int(binary.BigEndian.Uint16(b[i+2 : i+4]))
		if size < 2 || i+2+size > len(b) {
			return 1
		}
		seg := b[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) >= 6 && bytes.Equal(seg[:6], []byte("Exif\x00\x00")) {
			return tiffOrientation(seg[6:])
		}
		i += 2 + size
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	ifd := int(bo.Uint32(t[4:8]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[ifd : ifd+2]))
	for k := range n {
		e := ifd + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:e+2]) == 0x0112 && bo.Uint16(t[e+2:e+4]) == 3 { // SHORT
			if o := int(bo.Uint16(t[e+8 : e+10])); o >= 1 && o <= 8 {
				return o
			}
			return 1
		}
	}
	return 1
}
