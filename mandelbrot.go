package maps

import (
	"image"
	"image/color"
	"image/color/palette"
	"math/cmplx"
)

// Mandelbrot is a TileServer which renders an image of the Mandelbrot set.
type Mandelbrot struct {
	Palette color.Palette
}

// Get returns a tile for the given tile coordinates.
func (m Mandelbrot) Get(z, x, y int) (Tile, error) {
	x, y = normalizeTile(z, x, y)
	colors := m.Palette
	if colors == nil {
		colors = palette.Plan9
	}

	mandelbrot := func(c complex128) color.Color {
		z := complex(0, 0)
		idx := 0
		for i := 0; i < len(colors); i++ {
			z = z*z + c
			if cmplx.Abs(z) > 2 {
				idx = i
				break
			}
		}
		//log.Println(c, idx)
		return colors[idx]
	}

	// For full scale (z=0), let both real and imaginary part range from -1 to 1.
	c0 := complex(-1, -1)
	scale := 1 / (128 * two[z])

	var c complex128
	im := image.NewRGBA(image.Rect(0, 0, 256, 256))
	i0 := x * 256
	k0 := y * 256
	for i := 0; i < 256; i++ {
		for k := 0; k < 256; k++ {
			c = c0 + complex(scale*float64(i+i0), scale*float64(k+k0))
			im.Set(i, k, mandelbrot(c))
		}
	}
	return im, nil
}
