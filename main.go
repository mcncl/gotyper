package main

import "github.com/alecthomas/kong"

type Context struct {
	Debug bool
}

var cli struct {
	Debug bool `help: "Enable debug logging"`
}

func main() {
	ctx := kong.Parse(&cli, kong.UsageOnError())
	err := ctx.Run(&Context{Debug: cli.Debug})
	ctx.FatalIfErrorf(err)
}
