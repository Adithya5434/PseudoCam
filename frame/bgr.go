package frame

func (f *Frame) BGR24() []byte {
	width := f.Width
	height := f.Height

	ySize := width * height
	uvWidth := width / 2
	uvHeight := height / 2
	uvSize := uvWidth * uvHeight

	yPlane := f.Data[:ySize]
	uPlane := f.Data[ySize : ySize+uvSize]
	vPlane := f.Data[ySize+uvSize : ySize+uvSize*2]

	bgr := make([]byte, width*height*3)

	for y := 0; y < height; y++ {
		yRow := y * width
		uvRow := (y / 2) * uvWidth

		for x := 0; x < width; x++ {
			yValue := int(yPlane[yRow+x])
			uValue := int(uPlane[uvRow+x/2])
			vValue := int(vPlane[uvRow+x/2])

			c := yValue - 16
			d := uValue - 128
			e := vValue - 128

			if c < 0 {
				c = 0
			}

			r := (298*c + 409*e + 128) >> 8
			g := (298*c - 100*d - 208*e + 128) >> 8
			b := (298*c + 516*d + 128) >> 8

			if r < 0 {
				r = 0
			} else if r > 255 {
				r = 255
			}

			if g < 0 {
				g = 0
			} else if g > 255 {
				g = 255
			}

			if b < 0 {
				b = 0
			} else if b > 255 {
				b = 255
			}

			index := (yRow + x) * 3

			bgr[index] = byte(b)
			bgr[index+1] = byte(g)
			bgr[index+2] = byte(r)
		}
	}

	return bgr
}
