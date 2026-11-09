package main

import "embed"

//go:embed assets/ui.html
var page string

//go:embed assets/logo.png
var logo []byte

//go:embed THIRD_PARTY_NOTICES.txt
var thirdPartyNotices string

//go:embed assets/console.js
var consoleScript []byte

//go:embed assets/vendor/*
var terminalAssets embed.FS
