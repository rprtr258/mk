package markdown

import (
	"fmt"
	"io"
	"strings"
)

func Header(w io.Writer, level int, text string) {
	sharps := strings.Repeat("#", level)
	fmt.Fprintf(w, "%s %s\n", sharps, text)
}

func H1(w io.Writer, text string) {
	Header(w, 1, text)
}

func H2(w io.Writer, text string) {
	Header(w, 2, text) //nolint:gomnd // H2
}

func H3(w io.Writer, text string) {
	Header(w, 3, text) //nolint:gomnd // H3
}

func Code(w io.Writer, lang, code string) {
	fmt.Fprintf(w, "```%s\n%s\n```\n", lang, code)
}

func Table(w io.Writer, headers []string, rows [][]string) {
	for _, header := range headers {
		fmt.Fprint(w, "|")
		fmt.Fprint(w, header)
	}
	fmt.Fprint(w, "|\n")

	fmt.Fprint(w, strings.Repeat("|-", len(headers)))
	fmt.Fprint(w, "|\n")

	for _, rows := range rows {
		for _, cell := range rows {
			fmt.Fprint(w, "|")
			fmt.Fprint(w, cell)
		}
		fmt.Fprint(w, "|\n")
	}
}
