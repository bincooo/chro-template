package helper

import _ "embed"

var (
	// https://github.com/requireCool/stealth.min.js/blob/main/stealth.min.js
	//go:embed js/stealth.min.js
	stealthJs string
)
