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

var SequencerIPFlag = cli.StringFlag{
	Name:  "sequencer_ip",
	Usage: "specify sequencer_ip which will override the one in config",
}
