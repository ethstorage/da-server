```bash
$ cat config.json

{
	"SequencerIP": "127.0.0.1",
	"ListenAddr": "0.0.0.0:8888",
	"StorePath":  "/root/da/data"
}

$ go run main.go da start --config config.json
```
