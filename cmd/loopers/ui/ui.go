package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	// Clear the primary screen buffer robustly
	ClearScreen()

	// The logo mark is a 2×2 grid with a checkerboard pattern of solid and hollow squares
	// [Solid]  [Hollow]
	// [Solid]  [Solid]
	solidBlock := lipgloss.NewStyle().Foreground(colorTextPrimary).Bold(true).Render("[x]")
	hollowBlock := lipgloss.NewStyle().Foreground(colorTextTertiary).Render("[ ]")

	gap := " "

	row1Logo := solidBlock + gap + hollowBlock
	row2Logo := solidBlock + gap + solidBlock

	title := lipgloss.NewStyle().Foreground(colorTextPrimary).Bold(true).Render("LOOPERS")
	subtitle := lipgloss.NewStyle().Foreground(colorTextSecondary).Render("PRE-CALL AI BILLING CIRCUIT BREAKER  ·  (Press Ctrl+C to go back)")

	row1 := "  " + row1Logo + "   " + title
	row2 := "  " + row2Logo + "   " + subtitle

	fmt.Println()
	fmt.Println(row1)
	fmt.Println(row2)
	fmt.Println()
}
