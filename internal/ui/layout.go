package ui

import "fyne.io/fyne/v2"

// formLayout lays children out as a two-column key/value grid. Even
// indices are keys (right-aligned label width = max key width); odd
// indices are values stretched to fill the remaining width.
type formLayout struct{}

func layoutForm() fyne.Layout { return formLayout{} }

func (formLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	keyW, valueW, h := float32(0), float32(0), float32(0)
	const pad = float32(8)
	for i := 0; i < len(objects); i += 2 {
		ks := objects[i].MinSize()
		if ks.Width > keyW {
			keyW = ks.Width
		}
		var vs fyne.Size
		if i+1 < len(objects) {
			vs = objects[i+1].MinSize()
			if vs.Width > valueW {
				valueW = vs.Width
			}
		}
		row := ks.Height
		if vs.Height > row {
			row = vs.Height
		}
		h += row + pad
	}
	return fyne.NewSize(keyW+valueW+pad*2, h)
}

func (formLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	const pad = float32(8)
	keyW := float32(0)
	for i := 0; i < len(objects); i += 2 {
		if w := objects[i].MinSize().Width; w > keyW {
			keyW = w
		}
	}
	y := float32(0)
	for i := 0; i < len(objects); i += 2 {
		k := objects[i]
		ks := k.MinSize()
		var vs fyne.Size
		if i+1 < len(objects) {
			vs = objects[i+1].MinSize()
		}
		row := ks.Height
		if vs.Height > row {
			row = vs.Height
		}
		k.Move(fyne.NewPos(0, y))
		k.Resize(fyne.NewSize(keyW, row))
		if i+1 < len(objects) {
			v := objects[i+1]
			v.Move(fyne.NewPos(keyW+pad, y))
			v.Resize(fyne.NewSize(size.Width-keyW-pad, row))
		}
		y += row + pad
	}
}
