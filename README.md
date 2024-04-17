```bash
$ cat config.json

{
	"Sequencer": "0x53e80F8Cf25F8c0E93037EaF808C3A68C8440988",
	"ListenAddr": "0.0.0.0:8888",
	"StorePath":  "/root/da/data"
}

$ go run main.go da start --config config.json
```
