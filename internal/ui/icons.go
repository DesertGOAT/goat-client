package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/dlf-dds/goat-client/internal/ipc"
)

// Tray icons are generated at init from solid-color filled circles. We do
// this rather than checking in PNG/ICO assets so the package stays
// self-contained and so no netbird-trademarked icon art needs preserving.
// The convention follows snitch-app design doc §3: green = healthy,
// amber = transitional, red = error, gray = idle/disconnected.
var (
	iconDisconnected []byte
	iconConnecting   []byte
	iconConnected    []byte
	iconError        []byte
)

func init() {
	iconDisconnected = renderTrayIcon(color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff})
	iconConnecting = renderTrayIcon(color.RGBA{R: 0xe6, G: 0x9f, B: 0x00, A: 0xff})
	iconConnected = renderTrayIcon(color.RGBA{R: 0x2c, G: 0xa0, B: 0x2c, A: 0xff})
	iconError = renderTrayIcon(color.RGBA{R: 0xc0, G: 0x39, B: 0x2b, A: 0xff})
}

// iconForState picks the tray icon byte buffer matching `state`.
func iconForState(state ipc.State) []byte {
	switch state {
	case ipc.StateConnected:
		return iconConnected
	case ipc.StateConnecting:
		return iconConnecting
	case ipc.StateError:
		return iconError
	default:
		return iconDisconnected
	}
}

// renderTrayIcon returns a 22x22 PNG with a filled circle of `c` on
// transparent background. 22 px matches the macOS template-icon target
// size and downscales cleanly on Linux/Windows trays.
func renderTrayIcon(c color.RGBA) []byte {
	const size = 22
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size-1)/2, float64(size-1)/2
	r := float64(size)/2 - 1.5
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			d := dx*dx + dy*dy
			if d <= r*r {
				img.Set(x, y, c)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		// init-time encode of a tiny in-memory image cannot fail
		// outside of OOM; if it does, fall through with a 1x1 swatch
		// rather than panic so the tray still surfaces something.
		fallback := image.NewRGBA(image.Rect(0, 0, 1, 1))
		fallback.Set(0, 0, c)
		buf.Reset()
		_ = png.Encode(&buf, fallback)
	}
	return buf.Bytes()
}
