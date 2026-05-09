package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/dlf-dds/goat-client/internal/ipc"
)

// statusPane renders the single-tunnel status surface — interface state,
// last handshake, byte counters, peer pubkey, configured endpoints. There
// is exactly one tunnel (wg-cp0) so this is a flat key/value display, not
// a list view.
type statusPane struct {
	client ipc.Client

	state         *widget.Label
	interfaceName *widget.Label
	lastHandshake *widget.Label
	bytesIn       *widget.Label
	bytesOut      *widget.Label
	peerPubKey    *widget.Label
	endpoints     *widget.Label
	errorMessage  *widget.Label

	root fyne.CanvasObject
}

func newStatusPane(client ipc.Client) *statusPane {
	p := &statusPane{
		client:        client,
		state:         widget.NewLabel("—"),
		interfaceName: widget.NewLabel("—"),
		lastHandshake: widget.NewLabel("—"),
		bytesIn:       widget.NewLabel("—"),
		bytesOut:      widget.NewLabel("—"),
		peerPubKey:    widget.NewLabel("—"),
		endpoints:     widget.NewLabel("—"),
		errorMessage:  widget.NewLabel(""),
	}
	p.errorMessage.Wrapping = fyne.TextWrapWord
	p.peerPubKey.Wrapping = fyne.TextWrapBreak
	p.endpoints.Wrapping = fyne.TextWrapWord

	form := container.New(layoutForm(),
		widget.NewLabel("State:"), p.state,
		widget.NewLabel("Interface:"), p.interfaceName,
		widget.NewLabel("Last handshake:"), p.lastHandshake,
		widget.NewLabel("Bytes in:"), p.bytesIn,
		widget.NewLabel("Bytes out:"), p.bytesOut,
		widget.NewLabel("Peer pubkey:"), p.peerPubKey,
		widget.NewLabel("Endpoints:"), p.endpoints,
	)

	refresh := widget.NewButton("Refresh", func() { p.Refresh() })
	p.root = container.NewBorder(nil, container.NewVBox(p.errorMessage, refresh), nil, nil, form)
	p.Refresh()
	return p
}

func (p *statusPane) Content() fyne.CanvasObject { return p.root }

func (p *statusPane) Refresh() {
	if p.client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	st, err := p.client.GetStatus(ctx)
	if err != nil {
		p.errorMessage.SetText("Failed to fetch status: " + err.Error())
		return
	}
	p.errorMessage.SetText("")
	p.apply(st)
}

func (p *statusPane) apply(st *ipc.StatusInfo) {
	p.state.SetText(stateLabel(st.State))
	p.interfaceName.SetText(orDash(st.InterfaceName))
	if st.LastHandshake.IsZero() {
		p.lastHandshake.SetText("never")
	} else {
		p.lastHandshake.SetText(fmt.Sprintf("%s (%s ago)",
			st.LastHandshake.Format(time.RFC3339),
			time.Since(st.LastHandshake).Round(time.Second)))
	}
	p.bytesIn.SetText(fmt.Sprintf("%d", st.BytesIn))
	p.bytesOut.SetText(fmt.Sprintf("%d", st.BytesOut))
	p.peerPubKey.SetText(orDash(st.PeerPubKey))
	if len(st.Endpoints) == 0 {
		p.endpoints.SetText("—")
	} else {
		p.endpoints.SetText(strings.Join(st.Endpoints, "\n"))
	}
	if st.ErrorMessage != "" {
		p.errorMessage.SetText(st.ErrorMessage)
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
