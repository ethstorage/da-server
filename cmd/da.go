package cmd

import (
	"encoding/json"
	"os"

	"github.com/blockchaindevsh/da-server/cmd/flag"
	"github.com/blockchaindevsh/da-server/pkg/da"
	"github.com/urfave/cli"
)

// DACmd ...
var DACmd = cli.Command{
	Name:  "da",
	Usage: "da actions",
	Subcommands: []cli.Command{
		daStartCmd,
	},
}

var daStartCmd = cli.Command{
	Name:  "start",
	Usage: "start da server",
	Flags: []cli.Flag{
		flag.ConfigFlag,
	},
	Action: daStart,
}

func daStart(ctx *cli.Context) (err error) {
	configBytes, err := os.ReadFile(ctx.String(flag.ConfigFlag.Name))
	if err != nil {
		return
	}

	var config da.Config
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return
	}

	return
}
