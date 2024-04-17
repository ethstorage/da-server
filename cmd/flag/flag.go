package flag

import "github.com/urfave/cli"

var ConfigFlag = cli.StringFlag{
	Name:  "config",
	Usage: "specify config file",
}
