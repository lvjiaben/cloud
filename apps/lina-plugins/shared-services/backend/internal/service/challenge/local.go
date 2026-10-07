package challenge

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/big"
)

const (
	localImageWidth  = 220
	localImageHeight = 64
	glyphScale       = 3
)

var glyphs = map[rune][7]string{
	'0': {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "10000", "11110", "00001", "00001", "11110"},
	'6': {"01110", "10000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
	'+': {"00000", "00100", "00100", "11111", "00100", "00100", "00000"},
	'=': {"00000", "00000", "11111", "00000", "11111", "00000", "00000"},
	'?': {"01110", "10001", "00001", "00010", "00100", "00000", "00100"},
	' ': {},
}

func createLocalPresentation(accessible bool) (answer string, image []byte, prompt string, err error) {
	left, err := randomBetween(11, 89)
	if err != nil {
		return "", nil, "", err
	}
	right, err := randomBetween(10, 59)
	if err != nil {
		return "", nil, "", err
	}
	answer = fmt.Sprintf("%d", left+right)
	equation := fmt.Sprintf("%d + %d = ?", left, right)
	if accessible {
		return answer, nil, equation, nil
	}
	imageBytes, err := renderEquationPNG(equation)
	if err != nil {
		return "", nil, "", err
	}
	return answer, imageBytes, "", nil
}

func renderEquationPNG(equation string) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, localImageWidth, localImageHeight))
	fill(canvas, color.RGBA{R: 244, G: 247, B: 251, A: 255})
	if err := addNoise(canvas); err != nil {
		return nil, err
	}
	runes := []rune(equation)
	const glyphAdvance = 6 * glyphScale
	x := (localImageWidth - (len(runes)*glyphAdvance - glyphScale)) / 2
	for _, symbol := range runes {
		yOffset, err := randomBetween(-2, 2)
		if err != nil {
			return nil, err
		}
		drawGlyph(canvas, symbol, x, 21+yOffset, color.RGBA{R: 23, G: 32, B: 51, A: 255})
		x += glyphAdvance
	}
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func fill(canvas *image.RGBA, value color.RGBA) {
	for y := canvas.Rect.Min.Y; y < canvas.Rect.Max.Y; y++ {
		for x := canvas.Rect.Min.X; x < canvas.Rect.Max.X; x++ {
			canvas.SetRGBA(x, y, value)
		}
	}
}

func addNoise(canvas *image.RGBA) error {
	for range 6 {
		y1, err := randomBetween(5, localImageHeight-6)
		if err != nil {
			return err
		}
		y2, err := randomBetween(5, localImageHeight-6)
		if err != nil {
			return err
		}
		drawLine(canvas, 4, y1, localImageWidth-5, y2, color.RGBA{R: 154, G: 169, B: 189, A: 255})
	}
	for range 180 {
		x, err := randomBetween(1, localImageWidth-2)
		if err != nil {
			return err
		}
		y, err := randomBetween(1, localImageHeight-2)
		if err != nil {
			return err
		}
		canvas.SetRGBA(x, y, color.RGBA{R: 122, G: 142, B: 168, A: 255})
	}
	return nil
}

func drawGlyph(canvas *image.RGBA, symbol rune, startX, startY int, value color.RGBA) {
	rows, ok := glyphs[symbol]
	if !ok {
		rows = glyphs['?']
	}
	for row, pixels := range rows {
		for column, pixel := range pixels {
			if pixel != '1' {
				continue
			}
			for dy := range glyphScale {
				for dx := range glyphScale {
					canvas.SetRGBA(startX+column*glyphScale+dx, startY+row*glyphScale+dy, value)
				}
			}
		}
	}
}

func drawLine(canvas *image.RGBA, x0, y0, x1, y1 int, value color.RGBA) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	stepX, stepY := -1, -1
	if x0 < x1 {
		stepX = 1
	}
	if y0 < y1 {
		stepY = 1
	}
	errorValue := dx + dy
	for {
		canvas.SetRGBA(x0, y0, value)
		if x0 == x1 && y0 == y1 {
			return
		}
		doubled := 2 * errorValue
		if doubled >= dy {
			errorValue += dy
			x0 += stepX
		}
		if doubled <= dx {
			errorValue += dx
			y0 += stepY
		}
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func randomBetween(minimum int, maximum int) (int, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(int64(maximum-minimum+1)))
	if err != nil {
		return 0, err
	}
	return minimum + int(value.Int64()), nil
}
