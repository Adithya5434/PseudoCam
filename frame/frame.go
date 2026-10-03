package frame

type Frame struct {
	Width  int
	Height int
	Data   []byte
}

func New(width int, height int) *Frame {
	return &Frame{
		Width:  width,
		Height: height,
		Data:   make([]byte, width*height*3/2),
	}
}

func (f *Frame) Size() int {
	return f.Width * f.Height * 3/2
}

