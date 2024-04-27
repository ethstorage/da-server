package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethstorage/da-server/cmd/flag"
	"github.com/ethstorage/da-server/pkg/da"
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

	fmt.Println("config", string(configBytes))

	server := da.NewServer(&config)
	err = server.Start(context.Background())
	if err != nil {
		return
	}

	defer server.Stop(context.Background())

	signals := []os.Signal{
		os.Interrupt,
		os.Kill,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	}
	interruptChannel := make(chan os.Signal, 1)
	signal.Notify(interruptChannel, signals...)
	<-interruptChannel
	return
}
