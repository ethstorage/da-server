package flag

import "github.com/urfave/cli"

var ConfigFlag = cli.StringFlag{
	Name:  "config",
	Usage: "specify config file",
}

var RPCFlag = cli.StringFlag{
	Name:  "rpc",
	Usage: "specify rpc flag",
}

var BlobHashFlag = cli.StringFlag{
	Name:  "blob_hash",
	Usage: "specify blob hash flag",
}
