// Package qr encodes text as a QR Code (ISO/IEC 18004): byte mode, error
// correction level M, versions 1 to 20. It is what the emergency page
// shows for the sheet's link, without a third-party module. The encoder
// follows Project Nayuki's reference implementation.
package qr

import (
	"errors"
	"image"
	"image/color"
)

// MaxVersion is the largest version Encode builds: 20 holds 666 bytes at
// level M, far more than a sheet's link.
const MaxVersion = 20

// Code is an encoded symbol: Size×Size modules, true dark.
type Code struct {
	Size    int
	Version int
	Mask    int
	modules [][]bool
	fn      [][]bool // function modules, not masked
}

// Dark reports the module at column x, row y.
func (q *Code) Dark(x, y int) bool { return q.modules[y][x] }

// ErrTooLong is returned when text does not fit in MaxVersion.
var ErrTooLong = errors.New("teks terlalu panjang untuk QR")

// Level M per version (index 0 unused).
var eccPerBlock = [...]int{-1, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26}
var numBlocks = [...]int{-1, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16}

// Encode encodes data in the smallest version that holds it, with the mask
// of least penalty.
func Encode(data []byte) (*Code, error) { return encode(data, -1) }

// EncodeMask encodes with mask (0–7) instead of choosing one.
func EncodeMask(data []byte, mask int) (*Code, error) { return encode(data, mask) }

func encode(data []byte, mask int) (*Code, error) {
	version := 0
	for v := 1; v <= MaxVersion; v++ {
		if 4+countBits(v)+8*len(data) <= dataCodewords(v)*8 {
			version = v
			break
		}
	}
	if version == 0 {
		return nil, ErrTooLong
	}
	var bits []bool
	put := func(val, n int) {
		for i := n - 1; i >= 0; i-- {
			bits = append(bits, val>>i&1 != 0)
		}
	}
	put(4, 4) // byte mode
	put(len(data), countBits(version))
	for _, b := range data {
		put(int(b), 8)
	}
	capacity := dataCodewords(version) * 8
	put(0, min(4, capacity-len(bits)))
	put(0, (8-len(bits)%8)%8)
	for pad := 0xEC; len(bits) < capacity; pad ^= 0xEC ^ 0x11 {
		put(pad, 8)
	}
	words := make([]byte, len(bits)/8)
	for i, b := range bits {
		if b {
			words[i>>3] |= 1 << (7 - i&7)
		}
	}
	q := &Code{Version: version, Size: version*4 + 17}
	q.modules = grid(q.Size)
	q.fn = grid(q.Size)
	q.drawFunctions()
	q.drawCodewords(interleave(version, words))
	if mask < 0 {
		best := 1 << 30
		for m := 0; m < 8; m++ {
			q.applyMask(m)
			q.drawFormat(m)
			if p := q.penalty(); p < best {
				best, mask = p, m
			}
			q.applyMask(m)
		}
	}
	q.Mask = mask
	q.applyMask(mask)
	q.drawFormat(mask)
	return q, nil
}

func grid(n int) [][]bool {
	g := make([][]bool, n)
	for i := range g {
		g[i] = make([]bool, n)
	}
	return g
}

func countBits(version int) int {
	if version <= 9 {
		return 8
	}
	return 16
}

func rawModules(v int) int {
	r := (16*v+128)*v + 64
	if v >= 2 {
		n := v/7 + 2
		r -= (25*n-10)*n - 55
		if v >= 7 {
			r -= 36
		}
	}
	return r
}

func dataCodewords(v int) int { return rawModules(v)/8 - eccPerBlock[v]*numBlocks[v] }

func interleave(v int, data []byte) []byte {
	blocks, ecc := numBlocks[v], eccPerBlock[v]
	raw := rawModules(v) / 8
	short := blocks - raw%blocks
	shortLen := raw / blocks
	div := rsDivisor(ecc)
	var all [][]byte
	k := 0
	for i := 0; i < blocks; i++ {
		n := shortLen - ecc
		if i >= short {
			n++
		}
		d := append([]byte{}, data[k:k+n]...)
		k += n
		e := rsRemainder(d, div)
		if i < short {
			d = append(d, 0)
		}
		all = append(all, append(d, e...))
	}
	var out []byte
	for i := 0; i < len(all[0]); i++ {
		for j, b := range all {
			if i != shortLen-ecc || j >= short {
				out = append(out, b[i])
			}
		}
	}
	return out
}

func rsMul(x, y byte) byte {
	z := 0
	for i := 7; i >= 0; i-- {
		z = z<<1 ^ (z>>7)*0x11D
		z ^= int(y>>i&1) * int(x)
	}
	return byte(z)
}

func rsDivisor(degree int) []byte {
	r := make([]byte, degree)
	r[degree-1] = 1
	root := byte(1)
	for i := 0; i < degree; i++ {
		for j := range r {
			r[j] = rsMul(r[j], root)
			if j+1 < len(r) {
				r[j] ^= r[j+1]
			}
		}
		root = rsMul(root, 2)
	}
	return r
}

func rsRemainder(data, div []byte) []byte {
	r := make([]byte, len(div))
	for _, b := range data {
		f := b ^ r[0]
		copy(r, r[1:])
		r[len(r)-1] = 0
		for i := range r {
			r[i] ^= rsMul(div[i], f)
		}
	}
	return r
}

func (q *Code) set(x, y int, dark bool) {
	q.modules[y][x] = dark
	q.fn[y][x] = true
}

func (q *Code) drawFunctions() {
	for i := 0; i < q.Size; i++ {
		q.set(6, i, i%2 == 0)
		q.set(i, 6, i%2 == 0)
	}
	q.finder(3, 3)
	q.finder(q.Size-4, 3)
	q.finder(3, q.Size-4)
	pos := q.alignPositions()
	n := len(pos)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == 0 && j == 0 || i == 0 && j == n-1 || i == n-1 && j == 0 {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					q.set(pos[i]+dx, pos[j]+dy, max(abs(dx), abs(dy)) != 1)
				}
			}
		}
	}
	q.drawFormat(0) // reserves the format modules
	if q.Version >= 7 {
		rem := q.Version
		for i := 0; i < 12; i++ {
			rem = rem<<1 ^ (rem>>11)*0x1F25
		}
		bits := q.Version<<12 | rem
		for i := 0; i < 18; i++ {
			dark := bits>>i&1 != 0
			a, b := q.Size-11+i%3, i/3
			q.set(a, b, dark)
			q.set(b, a, dark)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (q *Code) finder(cx, cy int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			x, y := cx+dx, cy+dy
			if x < 0 || x >= q.Size || y < 0 || y >= q.Size {
				continue
			}
			d := max(abs(dx), abs(dy))
			q.set(x, y, d != 2 && d != 4)
		}
	}
}

func (q *Code) alignPositions() []int {
	if q.Version == 1 {
		return nil
	}
	n := q.Version/7 + 2
	step := (q.Version*8 + n*3 + 5) / (n*4 - 4) * 2
	out := make([]int, n)
	out[0] = 6
	for i, p := n-1, q.Size-7; i >= 1; i, p = i-1, p-step {
		out[i] = p
	}
	return out
}

func (q *Code) drawFormat(mask int) {
	data := 0<<3 | mask // level M is 00
	rem := data
	for i := 0; i < 10; i++ {
		rem = rem<<1 ^ (rem>>9)*0x537
	}
	bits := (data<<10 | rem) ^ 0x5412
	bit := func(i int) bool { return bits>>i&1 != 0 }
	for i := 0; i <= 5; i++ {
		q.set(8, i, bit(i))
	}
	q.set(8, 7, bit(6))
	q.set(8, 8, bit(7))
	q.set(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		q.set(14-i, 8, bit(i))
	}
	for i := 0; i < 8; i++ {
		q.set(q.Size-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		q.set(8, q.Size-15+i, bit(i))
	}
	q.set(8, q.Size-8, true)
}

func (q *Code) drawCodewords(data []byte) {
	i := 0
	for right := q.Size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < q.Size; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				upward := (right+1)&2 == 0
				y := vert
				if upward {
					y = q.Size - 1 - vert
				}
				if !q.fn[y][x] && i < len(data)*8 {
					q.modules[y][x] = data[i>>3]>>(7-i&7)&1 != 0
					i++
				}
			}
		}
	}
}

func (q *Code) applyMask(mask int) {
	for y := 0; y < q.Size; y++ {
		for x := 0; x < q.Size; x++ {
			var flip bool
			switch mask {
			case 0:
				flip = (x+y)%2 == 0
			case 1:
				flip = y%2 == 0
			case 2:
				flip = x%3 == 0
			case 3:
				flip = (x+y)%3 == 0
			case 4:
				flip = (x/3+y/2)%2 == 0
			case 5:
				flip = x*y%2+x*y%3 == 0
			case 6:
				flip = (x*y%2+x*y%3)%2 == 0
			case 7:
				flip = ((x+y)%2+x*y%3)%2 == 0
			}
			if flip && !q.fn[y][x] {
				q.modules[y][x] = !q.modules[y][x]
			}
		}
	}
}

// penalty scores the symbol as the standard's four rules do.
func (q *Code) penalty() int {
	n := q.Size
	result := 0
	line := func(get func(i int) bool) {
		run, color := 0, false
		var hist [7]int
		add := func(r int) {
			if hist[0] == 0 {
				r += n // the light border before the line
			}
			copy(hist[1:], hist[:6])
			hist[0] = r
		}
		count := func() int {
			c := hist[1]
			core := c > 0 && hist[2] == c && hist[3] == c*3 && hist[4] == c && hist[5] == c
			r := 0
			if core && hist[0] >= c*4 && hist[6] >= c {
				r++
			}
			if core && hist[6] >= c*4 && hist[0] >= c {
				r++
			}
			return r
		}
		for i := 0; i < n; i++ {
			if get(i) == color {
				run++
				if run == 5 {
					result += 3
				} else if run > 5 {
					result++
				}
				continue
			}
			add(run)
			if !color {
				result += count() * 40
			}
			color = get(i)
			run = 1
		}
		// terminate the line with the light border
		if color {
			add(run)
			run = 0
		}
		run += n
		add(run)
		result += count() * 40
	}
	for y := 0; y < n; y++ {
		line(func(x int) bool { return q.modules[y][x] })
	}
	for x := 0; x < n; x++ {
		line(func(y int) bool { return q.modules[y][x] })
	}
	dark := 0
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if q.modules[y][x] {
				dark++
			}
			if x < n-1 && y < n-1 {
				c := q.modules[y][x]
				if c == q.modules[y][x+1] && c == q.modules[y+1][x] && c == q.modules[y+1][x+1] {
					result += 3
				}
			}
		}
	}
	total := n * n
	k := (abs(dark*20-total*10)+total-1)/total - 1
	return result + k*10
}

// Image draws the symbol with a quiet zone of border modules, each scale
// pixels wide.
func (q *Code) Image(scale, border int) *image.Gray {
	side := (q.Size + 2*border) * scale
	img := image.NewGray(image.Rect(0, 0, side, side))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for y := 0; y < q.Size; y++ {
		for x := 0; x < q.Size; x++ {
			if !q.modules[y][x] {
				continue
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					img.SetGray((x+border)*scale+dx, (y+border)*scale+dy, color.Gray{})
				}
			}
		}
	}
	return img
}
