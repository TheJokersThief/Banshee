package main

import "github.com/charmbracelet/lipgloss"

var FatalErrorStyling = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#CD4B41")).
	BorderStyle(lipgloss.DoubleBorder()).
	BorderForeground(lipgloss.Color("#CD4B41")).
	BorderTop(true).BorderBottom(true).
	PaddingTop(1).PaddingBottom(1).PaddingLeft(5).PaddingRight(5)
