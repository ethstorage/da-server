package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/blockchaindevsh/da-server/cmd"
	"github.com/urfave/cli"
)

func setupAPP() *cli.App {
	app := cli.NewApp()
	app.Usage = "DA Server"
	app.Copyright = "Copyright in 2024"
	app.Commands = []cli.Command{
		cmd.DACmd,
	}
	app.Flags = []cli.Flag{}
	app.Before = func(context *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())
		return nil
	}
	return app
}

func main() {
	if err := setupAPP().Run(os.Args); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
