package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Brand Colors (tryloopers.com aesthetic)
	colorTextPrimary   = lipgloss.Color("#ffffff")
	colorTextSecondary = lipgloss.Color("#a0a0a0")
	colorTextTertiary  = lipgloss.Color("#666666")
	colorBorderStrong  = lipgloss.Color("#333333")

	// Semantic Colors
	colorGreen  = lipgloss.Color("42")
	colorRed    = lipgloss.Color("196")
	colorYellow = lipgloss.Color("220")

	// Base Styles
	styleHeaderBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorderStrong).
				Padding(0, 1)

	styleHeaderTitle = lipgloss.NewStyle().
				Foreground(colorTextPrimary).
				Bold(true)

	styleSuccess = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleError   = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleWarn    = lipgloss.NewStyle().Foreground(colorYellow)
	styleInfo    = lipgloss.NewStyle().Foreground(colorTextTertiary)
	styleLabel   = lipgloss.NewStyle().Foreground(colorTextSecondary).Width(12)

	styleTableBorder = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(colorBorderStrong)

	styleTableHdr = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(colorBorderStrong).
			Foreground(colorTextPrimary).
			Bold(true)

	styleCardBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorderStrong).
			Padding(1, 2)

	styleSummaryPass = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorGreen).
				Padding(0, 1)

	styleSummaryFail = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorRed).
				Padding(0, 1)
)

func PrintHeader(title string) {
	fmt.Println(styleHeaderBorder.Render(styleHeaderTitle.Render(title)))
}

func Success(msg string) {
	fmt.Printf("  %s  %s\n", styleSuccess.Render("✓"), msg)
}

func Error(msg string) {
	fmt.Printf("  %s  %s\n", styleError.Render("✗"), msg)
}

func Warn(msg string) {
	fmt.Printf("  %s  %s\n", styleWarn.Render("△"), msg)
}

func Info(msg string) {
	fmt.Printf("  %s  %s\n", styleInfo.Render("›"), styleInfo.Render(msg))
}

func PrintTable(headers []string, rows [][]string) {
	// Simple column width calculation
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, row := range rows {
		for i, c := range row {
			if w := lipgloss.Width(c); w > widths[i] {
				widths[i] = w
			}
		}
	}

	var out strings.Builder
	// Render headers
	for i, h := range headers {
		cell := lipgloss.NewStyle().Width(widths[i]).Render(h)
		out.WriteString(styleTableHdr.Render(cell))
		if i < len(headers)-1 {
			out.WriteString(styleInfo.Render(" │ "))
		}
	}
	out.WriteString("\n")

	// Render rows
	for _, row := range rows {
		for i, c := range row {
			cell := lipgloss.NewStyle().Width(widths[i]).Render(c)
			out.WriteString(cell)
			if i < len(row)-1 {
				out.WriteString(styleInfo.Render(" │ "))
			}
		}
		out.WriteString("\n")
	}

	fmt.Println(styleTableBorder.Render(out.String()))
}

func PrintBudgetBar(name string, spend, limit float64) {
	if limit == 0 {
		fmt.Printf("  %-8s %s\n", name, styleInfo.Render("not configured"))
		return
	}

	pct := (spend / limit) * 100
	barWidth := 20
	filled := int(math.Round((spend / limit) * float64(barWidth)))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}

	empty := barWidth - filled

	barColor := colorGreen
	if pct >= 90 {
		barColor = colorRed
	} else if pct >= 70 {
		barColor = colorYellow
	}

	barStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyStyle := lipgloss.NewStyle().Foreground(colorTextTertiary)

	bar := barStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", empty))

	stats := fmt.Sprintf("$%.2f / $%.2f  (%.0f%%)", spend, limit, pct)

	msg := fmt.Sprintf("  %-8s %s  %s", name, bar, stats)
	if pct > 100 {
		msg += styleError.Render(" △ OVER LIMIT")
	} else if pct >= 90 {
		msg += styleWarn.Render(" △ approaching limit")
	}

	fmt.Println(msg)
}

func PrintKeyCard(name, provider, rawKey, hash string) {
	var sb strings.Builder

	sb.WriteString(styleSuccess.Render("✓  Key Created Successfully") + "\n\n")
	sb.WriteString(fmt.Sprintf("%s%s\n", styleLabel.Render("Name"), name))
	sb.WriteString(fmt.Sprintf("%s%s\n\n", styleLabel.Render("Provider"), provider))
	sb.WriteString(fmt.Sprintf("%s%s\n", styleLabel.Render("Raw Key"), lipgloss.NewStyle().Foreground(colorTextPrimary).Bold(true).Render(rawKey)))
	sb.WriteString(fmt.Sprintf("%s%s\n\n", styleLabel.Render("Hash"), hash))

	warnLines := "△  Copy the raw key now. It won't be\n   shown again."
	sb.WriteString(styleWarn.Render(warnLines))

	fmt.Println(styleCardBorder.Render(sb.String()))
}

func PrintSummary(pass bool, issues int) {
	if pass {
		fmt.Println(styleSummaryPass.Render("✓  All systems go"))
	} else {
		msg := fmt.Sprintf("✗  Fix %d issues above", issues)
		fmt.Println(styleSummaryFail.Render(msg))
	}
}

func PrintLogo() {
	mark := "█ ·\n█ █"
	logo := lipgloss.NewStyle().Foreground(colorTextPrimary).Bold(true).Render(mark)
	text := lipgloss.NewStyle().Foreground(colorTextPrimary).Bold(true).PaddingLeft(2).PaddingTop(1).Render("LOOPERS")

	fmt.Println("\n" + lipgloss.JoinHorizontal(lipgloss.Top, logo, text) + "\n")
}

func GetHuhTheme() *huh.Theme {
	t := huh.ThemeBase()

	// Apply our monochrome palette
	t.Focused.Base = t.Focused.Base.BorderForeground(colorBorderStrong)
	t.Focused.Title = lipgloss.NewStyle().Foreground(colorTextPrimary).Bold(true)
	t.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(colorTextPrimary)
	t.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(colorTextPrimary)
	t.Focused.TextInput.Text = lipgloss.NewStyle().Foreground(colorTextPrimary)
	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(colorTextPrimary)
	t.Focused.Option = lipgloss.NewStyle().Foreground(colorTextPrimary)
	t.Focused.SelectedOption = lipgloss.NewStyle().Foreground(colorTextPrimary)
	t.Focused.UnselectedOption = lipgloss.NewStyle().Foreground(colorTextTertiary)

	t.Blurred.Title = lipgloss.NewStyle().Foreground(colorTextTertiary)
	t.Blurred.TextInput.Prompt = lipgloss.NewStyle().Foreground(colorTextTertiary)
	t.Blurred.TextInput.Text = lipgloss.NewStyle().Foreground(colorTextTertiary)

	t.Help.Ellipsis = lipgloss.NewStyle().Foreground(colorTextSecondary)
	t.Help.ShortKey = lipgloss.NewStyle().Foreground(colorTextSecondary)
	t.Help.ShortDesc = lipgloss.NewStyle().Foreground(colorTextTertiary)
	t.Help.ShortSeparator = lipgloss.NewStyle().Foreground(colorBorderStrong)

	return t
}
