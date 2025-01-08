package plugins

import _ "embed"

var (
	//go:embed nopecha.1
	Nopecha []byte

	//go:embed CaptchaSolver.1
	CaptchaSolver []byte

	// https://github.com/requireCool/stealth.min.js/blob/main/stealth.min.js
	//go:embed js/stealth.min.js
	StealthJs string

	//go:embed js/hook.js
	HookJs string
)
