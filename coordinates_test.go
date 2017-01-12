package maps

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"text/tabwriter"
)

func TestDegXY(t *testing.T) {
	for z := 0; z <= 24; z++ {
		for i := 0; i < 100; i++ {
			deg := Deg{
				2*MaxLat*rand.Float64() - MaxLat,
				360*rand.Float64() - 180,
			}
			xy := deg.XY(z)
			res := xy.Deg()
			if e := res.Distance(deg); e > 2*xy.PixelSize() {
				t.Errorf("distance too big after back transform: %s -> %s -> %s: error=%s @pixel resolution (%s/px)", deg, xy, res, e, xy.PixelSize())
			}
		}
	}
}

func TestXY(t *testing.T) {

}

func TestDistance(t *testing.T) {
	Cities["Darmstadt Stadtkirche"] = Deg{49.87139, 8.65631}
	Cities["Griesheim Lutherkirche"] = Deg{49.85987, 8.54996}
	Cities["North Pole"] = Deg{90, 0}
	Cities["South Pole"] = Deg{-90, 0}
	testCases := []struct {
		from, to string
		km       float64
	}{
		{"los angeles", "new york", 3935},
		{"new york", "los angeles", 3935},
		{"berlin", "munich", 502},
		{"hamburg", "berlin", 255},
		{"hamburg", "munich", 610},
		{"new york", "london", 5572},
		{"Darmstadt Stadtkirche", "Griesheim Lutherkirche", 7.74}, // End points for the base line of the "Grossherzoglich-Hessische Landvermessung 1808"
		{"North Pole", "South Pole", math.Pi * float64(EarthRadius) / 1000},
		{"cairo", "cairo", 0},
	}
	for _, tc := range testCases {
		from := Cities[tc.from]
		to := Cities[tc.to]
		distance := float64(from.Distance(to) / 1000)
		if e := math.Abs(distance - tc.km); e > 1 {
			t.Errorf("%s (%s) -> %s (%s): %.2f != %.2f", tc.from, from, tc.to, to, distance, tc.km)
		}
	}
}

func TestPixelSize(t *testing.T) {
	l := 2 * math.Pi * EarthRadius / 256
	var horizontalPixels uint64 = 256
	var buf bytes.Buffer
	tab := tabwriter.NewWriter(&buf, 0, 8, 2, ' ', 0)
	fmt.Fprintf(tab, "Z\tHor. Pixels\tPixelSize\n")
	for z := 0; z <= 24; z++ {
		xy := Deg{0, 0}.XY(z)
		s := xy.PixelSize()
		fmt.Fprintf(tab, "%d\t%d\t%v\n", z, horizontalPixels, s)
		if e := math.Abs(float64(l - s)); e > 1E-6 {
			t.Errorf("wrong PixelSize for zoom level %d. Got: %s instead of %s", z, s, l)
		}
		l /= 2
		horizontalPixels *= 2
	}
	tab.Flush()
	// fmt.Println(string(buf.Bytes())) // Create table for PixelSize documentation.
}
