package tile

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"text/tabwriter"
)

func TestLatLon_XY(t *testing.T) {
	checkXY := func(xy XY, err error, deg LatLon) {
		if err != nil {
			t.Fatal(err)
		}
		res := xy.LatLon()
		if e := res.Distance(deg); e > 2*xy.PixelSize() {
			t.Errorf("distance too big after back transform: %s -> %s -> %s: error=%s @pixel resolution (%s/px)", deg, xy, res, e, xy.PixelSize())
		}
	}

	for z := 0; z <= 24; z++ {
		// Transform random numbers
		for i := 0; i < 100; i++ {
			dLat := MaxLatitude - MinLatitude(z)
			deg := LatLon{
				dLat*Degree(rand.Float64()) + MinLatitude(z),
				Degree(360*rand.Float64() - 180),
			}
			xy, err := deg.XY(z)
			checkXY(xy, err, deg)
		}
	}
}

func ExampleXY() {
	for z := 0; z <= 24; z++ {
		tlXY := XY{X: 0, Y: 0, Z: z, XP: 0, YP: 0}
		brXY := XY{X: (1 << uint(z)) - 1, Y: (1 << uint(z)) - 1, Z: z, XP: 255, YP: 255}
		tlDeg := tlXY.LatLon()
		brDeg := brXY.LatLon()
		fmt.Printf("topLeft: %s = %s, bottomRight: %s = %s\n", tlXY, tlDeg, brXY, brDeg)
	}
}

func TestDistance(t *testing.T) {
	Cities["Darmstadt Stadtkirche"] = LatLon{49.87139, 8.65631}
	Cities["Griesheim Lutherkirche"] = LatLon{49.85987, 8.54996}
	Cities["North Pole"] = LatLon{90, 0}
	Cities["South Pole"] = LatLon{-90, 0}
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

func ExamplePixelSize() {
	l := 2 * math.Pi * EarthRadius / 256
	var horizontalPixels uint64 = 256
	var buf bytes.Buffer
	tab := tabwriter.NewWriter(&buf, 0, 8, 2, ' ', 0)
	fmt.Fprintf(tab, "Z\tHor. Pixels\tPixelSize\n")
	for z := 0; z <= 24; z++ {
		xy, err := LatLon{0, 0}.XY(z)
		if err != nil {
			panic(err)
		}
		s := xy.PixelSize()
		fmt.Fprintf(tab, "%d\t%d\t%v\n", z, horizontalPixels, s)
		if e := math.Abs(float64(l - s)); e > 1E-6 {
			panic(fmt.Sprintf("wrong PixelSize for zoom level %d. Got: %s instead of %s", z, s, l))
		}
		l /= 2
		horizontalPixels *= 2
	}
	tab.Flush()
	fmt.Println(string(buf.Bytes()))

	// Output:
	// Z   Hor. Pixels  PixelSize
	// 0   256          156368.08157840566m
	// 1   512          78184.04078920283m
	// 2   1024         39092.020394601415m
	// 3   2048         19546.010197300708m
	// 4   4096         9773.005098650354m
	// 5   8192         4886.502549325177m
	// 6   16384        2443.2512746625885m
	// 7   32768        1221.6256373312942m
	// 8   65536        610.8128186656471m
	// 9   131072       305.40640933282356m
	// 10  262144       152.70320466641178m
	// 11  524288       76.35160233320589m
	// 12  1048576      38.175801166602945m
	// 13  2097152      19.087900583301472m
	// 14  4194304      9.543950291650736m
	// 15  8388608      4.771975145825368m
	// 16  16777216     2.385987572912684m
	// 17  33554432     1.192993786456342m
	// 18  67108864     0.596496893228171m
	// 19  134217728    0.2982484466140855m
	// 20  268435456    0.14912422330704275m
	// 21  536870912    0.07456211165352138m
	// 22  1073741824   0.03728105582676069m
	// 23  2147483648   0.018640527913380344m
	// 24  4294967296   0.009320263956690172m
}
