package ui

import (
	"gioui.org/widget/material"
	"image/color"
)

func GenerateLightTheme() *ShipdonTheme {
	th := material.NewTheme()

	st := &ShipdonTheme{
		Theme: *th,
	}

	// light theme... leave it.
	return st
}

func GenerateDarkTheme() *ShipdonTheme {
	th := material.NewTheme()

	th.Bg = color.NRGBA{
		R: 64,
		G: 64,
		B: 64,
		A: 255,
	}

	th.Fg = color.NRGBA{
		R: 255,
		G: 255,
		B: 255,
		A: 255,
	}

	th.ContrastBg = color.NRGBA{
		R: 95,
		G: 147,
		B: 198,
		A: 255,
	}

	th.ContrastFg = color.NRGBA{
		R: 200,
		G: 200,
		B: 150,
		A: 255,
	}

	st := &ShipdonTheme{
		Theme: *th,
	}

	st.LinkColour = color.NRGBA{
		R: 76,
		G: 255,
		B: 0,
		A: 255,
	}

	st.IconBackgroundColour = color.NRGBA{
		R: 175,
		G: 175,
		B: 175,
		A: 255,
	}

	st.BoostedColour = color.NRGBA{
		R: 20,
		G: 121,
		B: 255,
		A: 255,
	}

	st.TitleBackgroundColour = color.NRGBA{
		R: 198,
		G: 198,
		B: 198,
		A: 255,
	}

	st.IconActiveColour = color.NRGBA{
		R: 0,
		G: 200,
		B: 0,
		A: 255,
	}

	st.IconInactiveColour = color.NRGBA{
		R: 200,
		G: 200,
		B: 0,
		A: 255,
	}

	st.StatusBackgroundColour = color.NRGBA{
		R: 60,
		G: 60,
		B: 60,
		A: 255,
	}

	return st
}
