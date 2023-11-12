package utils

import "strings"

func Escape(data string) string {
	res := data
	for _, symbol := range []string{"-", "]", "[", "{", "}", "(", ")", ">", "<", ".", "!", "*", "+", "=", "#", "~", "|", "`", "_"} {
		res = strings.ReplaceAll(res, symbol, "\\"+symbol)
	}

	return res
}
