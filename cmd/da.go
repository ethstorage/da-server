package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethstorage/da-server/cmd/flag"
	"github.com/ethstorage/da-server/pkg/da"
	"github.com/ethstorage/da-server/pkg/da/client"
	"github.com/urfave/cli"
)

// DACmd ...
var DACmd = cli.Command{
	Name:  "da",
	Usage: "da actions",
	Subcommands: []cli.Command{
		daStartCmd,
		daDownloadCmd,
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

var daDownloadCmd = cli.Command{
	Name:  "download",
	Usage: "download da blob from server",
	Flags: []cli.Flag{
		flag.RPCFlag,
		flag.BlobHashFlag,
	},
	Action: daDownload,
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

func getBlobHashes(blobHashesString string) (blobHashes []common.Hash) {
	for _, blobHashHex := range strings.Split(blobHashesString, ",") {
		blobHashes = append(blobHashes, common.HexToHash(blobHashHex))
	}
	return
}

func daDownload(ctx *cli.Context) (err error) {
	client := client.New(ctx.String(flag.RPCFlag.Name), common.Address{})
	blobHashes := getBlobHashes(ctx.String(flag.BlobHashFlag.Name))
	if len(blobHashes) == 0 {
		err = fmt.Errorf("none blob hash specified")
		return
	}
	blobs, err := client.GetBlobs(blobHashes)
	if err != nil {
		return
	}

	fmt.Println(blobs)
	return
}
